package report

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"inventory/internal/auth"
)

type fakeParser struct {
	userID uuid.UUID
	role   string
	perms  []string
	err    error
}

func (p fakeParser) ParseAccessToken(string) (uuid.UUID, string, []string, error) {
	if p.perms != nil {
		return p.userID, p.role, p.perms, p.err
	}
	return p.userID, p.role, auth.PermissionSetForRole(p.role), p.err
}

func setupEngine(repo Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(repo))
	r := gin.New()
	RegisterRoutes(r.Group("/api/v1"), h, fakeParser{role: "ADMIN"})
	return r
}

func doReq(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestStockSummaryOK(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(&StockSummary{
		Categories: []*CategorySummary{{Name: "Books", ProductCount: 2, TotalQty: 10, TotalValue: 250}},
		LowStock:   []*LowStockItem{},
	}, nil)
	m.On("CountProducts", mock.Anything).Return(int64(2), nil)
	m.On("InventoryValue", mock.Anything).Return(250.0, nil)

	w := doReq(t, setupEngine(m), "/api/v1/reports/stock-summary")
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "data")
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(2), data["total_products"])
	assert.Equal(t, float64(250), data["total_value"])
}

func TestStockSummaryServiceError(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(nil, errBoom)
	w := doReq(t, setupEngine(m), "/api/v1/reports/stock-summary")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestExportOKStreamsCSV(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(&StockSummary{
		Categories: []*CategorySummary{{Name: "Books, Modern", ProductCount: 2, TotalQty: 10, TotalValue: 250.5}},
		LowStock:   []*LowStockItem{},
	}, nil)
	m.On("CountProducts", mock.Anything).Return(int64(2), nil)
	m.On("InventoryValue", mock.Anything).Return(250.50, nil)

	w := doReq(t, setupEngine(m), "/api/v1/reports/export")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, w.Body.String(), "\uFEFFcategory,product_count,total_qty,total_value")
	assert.Contains(t, w.Body.String(), `"Books, Modern"`)
}

func TestExportServiceError(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(nil, errBoom)
	w := doReq(t, setupEngine(m), "/api/v1/reports/export")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestLowStockExportOK(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(&StockSummary{
		Categories: []*CategorySummary{},
		LowStock: []*LowStockItem{
			{ProductID: "pid-1", SKU: "SK", Name: "Widget", Category: "Tools", Quantity: 2, Threshold: 5, Value: 20},
		},
	}, nil)
	m.On("CountProducts", mock.Anything).Return(int64(1), nil)
	m.On("InventoryValue", mock.Anything).Return(20.0, nil)

	w := doReq(t, setupEngine(m), "/api/v1/reports/export-low-stock")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "product_id,sku,name,category,quantity,threshold,value")
	assert.Contains(t, w.Body.String(), "pid-1,SK,Widget,Tools,2,5,20.00")
}
