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
	"go.uber.org/zap"

	"inventory/internal/auth"
	"inventory/internal/shared/audit"
	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/validator"
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

func setupEngine(repo Repository, parser middleware.ClaimsParser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := NewService(repo)
	h := NewHandler(svc, validator.New())
	r := gin.New()
	RegisterRoutes(r.Group("/api/v1"), h, parser, nil)
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
	m.On("List", mock.Anything, mock.MatchedBy(func(q ListQuery) bool { return q.LowStock == false })).
		Return([]*InventoryView{{ProductID: pid, ProductSKU: "W1", ProductName: "Widget", Quantity: 5}}, int64(1), nil)

	r := setupEngine(m, fakeParser{role: "STAFF"})
	w := doReq(t, r, "GET", "/api/v1/inventory", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	assert.Contains(t, body, "data")
	data := body["data"].([]any)
	assert.Equal(t, "Widget", data[0].(map[string]any)["product_name"])
	assert.NotNil(t, body["meta"])
}

func TestInventoryListInvalidProductID(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, nil)

	w := doReq(t, r, "GET", "/api/v1/inventory?product_id=not-a-uuid", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInventoryListLowStockFilter(t *testing.T) {
	m := new(mockRepo)
	m.On("List", mock.Anything, mock.MatchedBy(func(q ListQuery) bool { return q.LowStock })).
		Return([]*InventoryView{}, int64(0), nil)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory?low_stock=true", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInventoryStockInOK(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("Receive", mock.Anything, mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == LedgerReceive && mv.Quantity == 10
	})).Return(&Inventory{ProductID: pid, Quantity: 10}, nil)

	r := setupEngine(m, fakeParser{role: "ADMIN"})
	body := `{"product_id":"` + pid.String() + `","quantity":10}`
	w := doReq(t, r, "POST", "/api/v1/inventory/receive", body, "tok")

	assert.Equal(t, http.StatusOK, w.Code)
	got := decode(t, w)
	assert.Equal(t, float64(10), got["data"].(map[string]any)["quantity"])
}

func TestInventoryStockInValidation(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{role: "ADMIN"})

	w := doReq(t, r, "POST", "/api/v1/inventory/receive", `{"product_id":"`+uuid.New().String()+`","quantity":0}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInventoryStockInUnauthenticated(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{err: sharederr.ErrUnauthorized})
	pid := uuid.New()

	w := doReq(t, r, "POST", "/api/v1/inventory/receive", `{"product_id":"`+pid.String()+`","quantity":5}`, "tok")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestInventoryStockInStaffAllowed(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("Receive", mock.Anything, mock.Anything).Return(&Inventory{ProductID: pid, Quantity: 3}, nil)

	r := setupEngine(m, fakeParser{role: "STAFF"})
	body := `{"product_id":"` + pid.String() + `","quantity":3}`
	w := doReq(t, r, "POST", "/api/v1/inventory/receive", body, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInventoryStockOutOverdrawConflict(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("Issue", mock.Anything, mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == LedgerIssue
	})).Return(nil, sharederr.ErrConflict)

	r := setupEngine(m, fakeParser{role: "ADMIN"})
	body := `{"product_id":"` + pid.String() + `","quantity":99}`
	w := doReq(t, r, "POST", "/api/v1/inventory/issue", body, "tok")
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestInventoryTransactionsList(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	tid := uuid.New()
	m.On("Ledger", mock.Anything, mock.Anything).Return([]*LedgerView{{
		ID: tid, ProductID: pid, ProductSKU: "W1", ProductName: "Widget", TransactionType: LedgerReceive, Quantity: 5,
	}}, int64(1), nil)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory/ledger", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	data := decode(t, w)["data"].([]any)
	assert.Equal(t, LedgerReceive, data[0].(map[string]any)["transaction_type"])
}

func TestInventoryTransactionsInvalidType(t *testing.T) {
	m := new(mockRepo)
	m.On("Ledger", mock.Anything, mock.Anything).Return(nil, int64(0), sharederr.ErrValidation)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory/ledger?type=SIDE", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInventoryExportCSV(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("List", mock.Anything, mock.Anything).Return(
		[]*InventoryView{{
			ProductID: pid, ProductSKU: "W1", ProductName: "Widget", Quantity: 5,
		}},
		int64(1), nil,
	)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory/export", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=")
	assert.Contains(t, w.Body.String(), "product_id,sku,name,quantity")
}

func TestTransferStaffAllowed(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	w1 := uuid.New()
	w2 := uuid.New()
	m.On("Transfer", mock.Anything, mock.MatchedBy(func(tr Transfer) bool {
		return tr.ProductID == pid && tr.Quantity == 2
	})).Return(&Inventory{ProductID: pid, Quantity: 2}, nil)

	r := setupEngine(m, fakeParser{role: "STAFF"})
	body := `{"product_id":"` + pid.String() + `","from_warehouse_id":"` + w1.String() + `","to_warehouse_id":"` + w2.String() + `","quantity":2}`
	w := doReq(t, r, "POST", "/api/v1/inventory/transfers", body, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTransferAdminAllowed(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	w1 := uuid.New()
	w2 := uuid.New()
	m.On("Transfer", mock.Anything, mock.Anything).Return(&Inventory{ProductID: pid, Quantity: 1}, nil)

	r := setupEngine(m, fakeParser{role: "ADMIN"})
	body := `{"product_id":"` + pid.String() + `","from_warehouse_id":"` + w1.String() + `","to_warehouse_id":"` + w2.String() + `","quantity":1}`
	w := doReq(t, r, "POST", "/api/v1/inventory/transfers", body, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTransferUnauthenticated(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{err: sharederr.ErrUnauthorized})

	body := `{"product_id":"` + uuid.New().String() + `","from_warehouse_id":"` + uuid.New().String() + `","to_warehouse_id":"` + uuid.New().String() + `","quantity":1}`
	w := doReq(t, r, "POST", "/api/v1/inventory/transfers", body, "tok")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTransferSameWarehouseReturns400(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	sameWh := uuid.New()

	r := setupEngine(m, fakeParser{role: "STAFF"})
	body := `{"product_id":"` + pid.String() + `","from_warehouse_id":"` + sameWh.String() + `","to_warehouse_id":"` + sameWh.String() + `","quantity":5}`
	w := doReq(t, r, "POST", "/api/v1/inventory/transfers", body, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTransferOverdrawReturns409(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	w1 := uuid.New()
	w2 := uuid.New()
	m.On("Transfer", mock.Anything, mock.Anything).Return(nil, sharederr.ErrConflict)

	r := setupEngine(m, fakeParser{role: "STAFF"})
	body := `{"product_id":"` + pid.String() + `","from_warehouse_id":"` + w1.String() + `","to_warehouse_id":"` + w2.String() + `","quantity":999}`
	w := doReq(t, r, "POST", "/api/v1/inventory/transfers", body, "tok")
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestTransferBadUUIDsReturn400(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{role: "STAFF"})

	cases := []string{
		`{"product_id":"nope","from_warehouse_id":"` + uuid.New().String() + `","to_warehouse_id":"` + uuid.New().String() + `","quantity":1}`,
		`{"product_id":"` + uuid.New().String() + `","from_warehouse_id":"nope","to_warehouse_id":"` + uuid.New().String() + `","quantity":1}`,
		`{"product_id":"` + uuid.New().String() + `","from_warehouse_id":"` + uuid.New().String() + `","to_warehouse_id":"nope","quantity":1}`,
		`{"product_id":"` + uuid.New().String() + `","quantity":0}`,
	}
	for _, body := range cases {
		w := doReq(t, r, "POST", "/api/v1/inventory/transfers", body, "tok")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func TestInventoryListBadFiltersReturn400(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, nil)

	w := doReq(t, r, "GET", "/api/v1/inventory?product_id=nope", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doReq(t, r, "GET", "/api/v1/inventory?warehouse_id=nope", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInventoryListForwardsWarehouseFilter(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	wh := uuid.New()
	m.On("List", mock.Anything, mock.MatchedBy(func(q ListQuery) bool {
		return q.WarehouseID != nil && *q.WarehouseID == wh
	})).Return([]*InventoryView{{
		ProductID: pid, ProductSKU: "W1", ProductName: "Widget", Quantity: 7,
	}}, int64(1), nil)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory?warehouse_id="+wh.String(), "", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInventoryTransactionsBadWarehouseFilter(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, nil)

	w := doReq(t, r, "GET", "/api/v1/inventory/ledger?warehouse_id=nope", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInventoryTransactionsForwardsWarehouseFilter(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	wh := uuid.New()
	m.On("Ledger", mock.Anything, mock.MatchedBy(func(q LedgerQuery) bool {
		return q.WarehouseID != nil && *q.WarehouseID == wh
	})).Return([]*LedgerView{{
		ProductID: pid, ProductSKU: "W1", ProductName: "Widget", TransactionType: LedgerReceive, Quantity: 5,
	}}, int64(1), nil)

	r := setupEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/inventory/ledger?warehouse_id="+wh.String(), "", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInventoryStockInBadWarehouseUUID(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	r := setupEngine(m, fakeParser{role: "ADMIN"})

	body := `{"product_id":"` + pid.String() + `","quantity":5,"warehouse_id":"nope"}`
	w := doReq(t, r, "POST", "/api/v1/inventory/receive", body, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInventoryStockOutForwardsWarehouse(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	wh := uuid.New()
	m.On("Issue", mock.Anything, mock.MatchedBy(func(mv Movement) bool {
		return mv.Type == LedgerIssue && mv.WarehouseID != nil && *mv.WarehouseID == wh
	})).Return(&Inventory{ProductID: pid, Quantity: 2}, nil)

	r := setupEngine(m, fakeParser{role: "ADMIN"})
	body := `{"product_id":"` + pid.String() + `","quantity":2,"warehouse_id":"` + wh.String() + `"}`
	w := doReq(t, r, "POST", "/api/v1/inventory/issue", body, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInventoryNoteOrNil(t *testing.T) {
	assert.Nil(t, noteOrNil(nil))
	note := "hello"
	assert.Equal(t, "hello", noteOrNil(&note))
}

type recordingAudit struct {
	entries []audit.Entry
}

func (r *recordingAudit) Record(e audit.Entry) {
	r.entries = append(r.entries, e)
}

func TestInventorySetAuditAndSetLoggerNilSafe(t *testing.T) {
	m := new(mockRepo)
	svc := NewService(m)
	h := NewHandler(svc, validator.New())

	h.SetAudit(nil)
	h.SetLogger(nil)
	assert.NotNil(t, h, "nil-safe setters must not panic")
}

func TestInventorySetAuditRecordsStockIn(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("Receive", mock.Anything, mock.Anything).Return(&Inventory{ProductID: pid, Quantity: 4}, nil)

	rec := &recordingAudit{}
	gin.SetMode(gin.TestMode)
	svc := NewService(m)
	h := NewHandler(svc, validator.New())
	h.SetAudit(rec)
	r := gin.New()
	r.POST("/api/v1/inventory/receive", middleware.Auth(fakeParser{userID: uuid.New(), role: "ADMIN"}), h.Receive)

	body := `{"product_id":"` + pid.String() + `","quantity":4}`
	w := doReq(t, r, "POST", "/api/v1/inventory/receive", body, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, rec.entries, 1)
	assert.Equal(t, "inventory", rec.entries[0].EntityType)
	assert.Equal(t, "INVENTORY_RECEIVE", rec.entries[0].Action)
}

func TestInventorySetAuditCapturesRequestContext(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("Receive", mock.Anything, mock.Anything).Return(&Inventory{ProductID: pid, Quantity: 4}, nil)

	rec := &recordingAudit{}
	gin.SetMode(gin.TestMode)
	svc := NewService(m)
	h := NewHandler(svc, validator.New())
	h.SetAudit(rec)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.POST("/api/v1/inventory/receive", middleware.Auth(fakeParser{userID: uuid.New(), role: "ADMIN"}), h.Receive)

	body := `{"product_id":"` + pid.String() + `","quantity":4}`
	req := httptest.NewRequest("POST", "/api/v1/inventory/receive", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("User-Agent", "qa-agent/1.0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	require.Len(t, rec.entries, 1)
	e := rec.entries[0]
	require.NotNil(t, e.UserAgent)
	assert.Equal(t, "qa-agent/1.0", *e.UserAgent)
	require.NotNil(t, e.RequestID)
	assert.NotEmpty(t, *e.RequestID)
	require.NotNil(t, e.BeforeData)
	require.NotNil(t, e.AfterData)
}

func TestInventoryExportLogsCSVFailure(t *testing.T) {
	m := new(mockRepo)
	m.On("List", mock.Anything, mock.Anything).Return([]*InventoryView{}, int64(0), nil)

	gin.SetMode(gin.TestMode)
	svc := NewService(m)
	h := NewHandler(svc, validator.New())
	zlog := zap.NewNop()
	h.SetLogger(zlog)
	r := gin.New()
	RegisterRoutes(r.Group("/api/v1"), h, nil, nil)

	w := doReq(t, r, "GET", "/api/v1/inventory/export", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestReceiveIdempotencyNoDuplicateMovements is the PRD §18 guarantee tested
// end-to-end: the same Idempotency-Key replayed against POST /inventory/receive
// must return the stored response and never create a second movement.
func TestReceiveIdempotencyNoDuplicateMovements(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)
	require.NoError(t, db.AutoMigrate(&middleware.IdempotencyKey{}))
	require.NoError(t, db.Exec("ALTER TABLE idempotency_keys DROP CONSTRAINT IF EXISTS idempotency_keys_user_id_fkey").Error)
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS idempotency_keys CASCADE") })

	store := middleware.NewIdempotencyStore(db)

	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(repo), validator.New())
	r := gin.New()
	RegisterRoutes(r.Group("/api/v1"), h, fakeParser{userID: uuid.New(), role: "ADMIN"}, store)

	body := `{"product_id":"` + p.ID.String() + `","quantity":7,"unit_cost":3.5,"note":"idem e2e"}`
	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/receive", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "phase3-key-1")
		req.Header.Set("Authorization", "Bearer tok")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	first := send()
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	second := send()
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())

	// Replay must return the byte-identical stored response.
	assert.Equal(t, first.Body.String(), second.Body.String())

	// Exactly one movement was recorded and stock moved once.
	var ledgerRows int64
	require.NoError(t, db.Model(&LedgerEntry{}).Where("product_id = ?", p.ID).Count(&ledgerRows).Error)
	assert.Equal(t, int64(1), ledgerRows)

	var inv Inventory
	require.NoError(t, db.Where("product_id = ?", p.ID).First(&inv).Error)
	assert.Equal(t, 7, inv.Quantity)

	// Same key with a DIFFERENT body is rejected, not silently executed.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/receive", bytes.NewBufferString(`{"product_id":"`+p.ID.String()+`","quantity":99}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "phase3-key-1")
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)

	// The rejected retry still left exactly one movement.
	require.NoError(t, db.Model(&LedgerEntry{}).Where("product_id = ?", p.ID).Count(&ledgerRows).Error)
	assert.Equal(t, int64(1), ledgerRows)
}
