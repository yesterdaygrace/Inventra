package dashboard

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

	sharederr "inventory/internal/shared/errors"
)

type fakeParser struct {
	userID uuid.UUID
	role   string
	err    error
}

func (p fakeParser) ParseAccessToken(string) (uuid.UUID, string, error) {
	return p.userID, p.role, p.err
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

func TestSummaryOK(t *testing.T) {
	m := new(mockRepo)
	m.On("CountProducts").Return(int64(3), nil)
	m.On("CountCategories").Return(int64(1), nil)
	m.On("InventoryValue").Return(100.0, nil)
	m.On("LowStockItems").Return([]*LowStockItem{}, nil)
	m.On("RecentActivities", RecentActivityLimit).Return([]*RecentActivity{}, nil)
	m.On("StockHealth").Return(StockHealth{}, nil)

	w := doReq(t, setupEngine(m), "/api/v1/dashboard/summary")
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(3), data["total_products"])
	assert.Equal(t, float64(1), data["total_categories"])
}

func TestActivityOK(t *testing.T) {
	m := new(mockRepo)
	m.On("Activities", 0, 0).Return([]*RecentActivity{{Action: "CREATE"}}, int64(1), nil)
	r := setupEngine(m)
	w := doReq(t, r, "/api/v1/dashboard/activity")
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body["success"].(bool))
	items := body["data"].([]any)
	require.Len(t, items, 1)
	pg := body["pagination"].(map[string]any)
	assert.Equal(t, float64(1), pg["total"])
}

func TestActivityServiceError(t *testing.T) {
	m := new(mockRepo)
	m.On("Activities", 0, 0).Return(nil, int64(0), errBoom)
	r := setupEngine(m)
	w := doReq(t, r, "/api/v1/dashboard/activity")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSummaryServiceError(t *testing.T) {
	m := new(mockRepo)
	m.On("CountProducts").Return(int64(0), errBoom)
	r := setupEngine(m)
	w := doReq(t, r, "/api/v1/dashboard/summary")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestInventoryMovementOK(t *testing.T) {
	m := new(mockRepo)
	m.On("InventoryMovement", mock.AnythingOfType("time.Time")).Return([]*DayMovement{}, nil)
	m.On("TotalQuantity").Return(int64(5), nil)

	r := setupEngine(m)
	w := doReq(t, r, "/api/v1/dashboard/inventory-movement?days=7")
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	payload := body["data"].(map[string]any)
	assert.Len(t, payload["labels"].([]any), 7)
	assert.Len(t, payload["datasets"].([]any), 4)
}

func TestInventoryMovementInvalidDays(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m)
	for _, days := range []string{"abc", "0", "-3"} {
		w := doReq(t, r, "/api/v1/dashboard/inventory-movement?days="+days)
		assert.Equal(t, http.StatusBadRequest, w.Code, "days=%q", days)
	}
}

func TestCategoryDistributionOK(t *testing.T) {
	m := new(mockRepo)
	m.On("CategoryDistribution").Return([]*CategoryCount{{Name: "Books", Count: 2}}, nil)
	r := setupEngine(m)
	w := doReq(t, r, "/api/v1/dashboard/category-distribution")
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	payload := body["data"].(map[string]any)
	labels := payload["labels"].([]any)
	assert.Equal(t, "Books", labels[0])
}

func TestTopSellingOK(t *testing.T) {
	m := new(mockRepo)
	m.On("TopSellers", 5).Return([]*TopSeller{{Name: "Widget", UnitsSold: 9}}, nil)
	r := setupEngine(m)
	w := doReq(t, r, "/api/v1/dashboard/top-selling")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTopSellingInvalidLimit(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m)
	w := doReq(t, r, "/api/v1/dashboard/top-selling?limit=zzz")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUnauthenticatedRejected(t *testing.T) {
	m := new(mockRepo)
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(m))
	r := gin.New()
	RegisterRoutes(r.Group("/api/v1"), h, fakeParser{err: sharederr.ErrUnauthorized})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMovementDaysDefaultAndReject(t *testing.T) {
	d, err := movementDays("")
	require.NoError(t, err)
	assert.Equal(t, DefaultMovementDays, d)
	for _, raw := range []string{"x", "0", "-1"} {
		if _, err := movementDays(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestTopLimitDefaultAndReject(t *testing.T) {
	l, err := topLimit("")
	require.NoError(t, err)
	assert.Equal(t, 5, l)
	if _, err := topLimit("nope"); err == nil {
		t.Fatal("expected error for non-numeric limit")
	}
}
