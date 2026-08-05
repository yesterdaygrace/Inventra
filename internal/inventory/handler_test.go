package inventory

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/validator"
)

type fakeParser struct {
	userID uuid.UUID
	role   string
	err    error
}

func (p fakeParser) ParseAccessToken(string) (uuid.UUID, string, error) {
	return p.userID, p.role, p.err
}

func setupEngine(repo Repository, parser middleware.ClaimsParser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := NewService(repo)
	h := NewHandler(svc, validator.New())
	r := gin.New()
	RegisterRoutes(r.Group("/api/v1"), h, parser)
	return r
}

func doReq(t *testing.T, r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestInventoryListPublic(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("List", mock.MatchedBy(func(q ListQuery) bool { return q.LowStock == false })).
		Return([]*InventoryView{{ProductID: pid, ProductSKU: "W1", ProductName: "Widget", Quantity: 5}}, int64(1), nil)

	r := setupEngine(m, fakeParser{role: "STAFF"})
	w := doReq(t, r, "GET", "/api/v1/inventory", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	assert.True(t, body["success"].(bool))
	data := body["data"].([]any)
	assert.Equal(t, "Widget", data[0].(map[string]any)["product_name"])
	assert.NotNil(t, body["pagination"])
}

func TestInventoryListInvalidProductID(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, nil)

	w := doReq(t, r, "GET", "/api/v1/inventory?product_id=not-a-uuid", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInventoryListLowStockFilter(t *testing.T) {
	m := new(mockRepo)
	m.On("List", mock.MatchedBy(func(q ListQuery) bool { return q.LowStock })).
		Return([]*InventoryView{}, int64(0), nil)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory?low_stock=true", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInventoryStockInOK(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("StockIn", mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == "IN" && mv.Quantity == 10
	})).Return(&Inventory{ProductID: pid, Quantity: 10}, nil)

	r := setupEngine(m, fakeParser{role: "ADMIN"})
	body := `{"product_id":"` + pid.String() + `","quantity":10}`
	w := doReq(t, r, "POST", "/api/v1/inventory/stock-in", body, "tok")

	assert.Equal(t, http.StatusOK, w.Code)
	got := decode(t, w)
	assert.Equal(t, float64(10), got["data"].(map[string]any)["quantity"])
}

func TestInventoryStockInValidation(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{role: "ADMIN"})

	w := doReq(t, r, "POST", "/api/v1/inventory/stock-in", `{"product_id":"`+uuid.New().String()+`","quantity":0}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInventoryStockInUnauthenticated(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{err: sharederr.ErrUnauthorized})
	pid := uuid.New()

	w := doReq(t, r, "POST", "/api/v1/inventory/stock-in", `{"product_id":"`+pid.String()+`","quantity":5}`, "tok")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestInventoryStockInStaffAllowed(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("StockIn", mock.Anything).Return(&Inventory{ProductID: pid, Quantity: 3}, nil)

	r := setupEngine(m, fakeParser{role: "STAFF"})
	body := `{"product_id":"` + pid.String() + `","quantity":3}`
	w := doReq(t, r, "POST", "/api/v1/inventory/stock-in", body, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInventoryStockOutOverdrawConflict(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("StockOut", mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == "OUT"
	})).Return(nil, sharederr.ErrConflict)

	r := setupEngine(m, fakeParser{role: "ADMIN"})
	body := `{"product_id":"` + pid.String() + `","quantity":99}`
	w := doReq(t, r, "POST", "/api/v1/inventory/stock-out", body, "tok")
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestInventoryTransactionsList(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	tid := uuid.New()
	m.On("Transactions", mock.Anything).Return([]*TransactionView{{
		ID: tid, ProductID: pid, ProductSKU: "W1", ProductName: "Widget", Type: "IN", Quantity: 5,
	}}, int64(1), nil)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory/transactions", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	data := decode(t, w)["data"].([]any)
	assert.Equal(t, "IN", data[0].(map[string]any)["type"])
}

func TestInventoryTransactionsInvalidType(t *testing.T) {
	m := new(mockRepo)
	m.On("Transactions", mock.Anything).Return(nil, int64(0), sharederr.ErrValidation)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory/transactions?type=SIDE", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInventoryExportCSV(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("List", mock.Anything).Return([]*InventoryView{{ProductID: pid, ProductSKU: "W1", ProductName: "Widget", Quantity: 5}}, int64(1), nil)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory/export", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=")
	assert.Contains(t, w.Body.String(), "product_id,sku,name,quantity")
}
