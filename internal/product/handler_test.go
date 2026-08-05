package product

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

func setupProductEngine(repo Repository, parser middleware.ClaimsParser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := NewService(repo)
	h := NewHandler(svc, validator.New())
	r := gin.New()
	group := r.Group("/api/v1")
	RegisterRoutes(group, h, parser)
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

func decodeProduct(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestListPublic(t *testing.T) {
	m := new(mockRepo)
	prods := []*Product{{ID: uuid.New(), Name: "Widget"}}
	m.On("List", mock.MatchedBy(func(q ListQuery) bool { return q.Q == "widget" })).
		Return(prods, int64(1), nil)

	r := setupProductEngine(m, fakeParser{role: "STAFF"})
	w := doReq(t, r, "GET", "/api/v1/products?q=widget", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeProduct(t, w)
	assert.True(t, body["success"].(bool))
	assert.NotNil(t, body["pagination"])
}

func TestCreateRequiresAdmin(t *testing.T) {
	m := new(mockRepo)
	r := setupProductEngine(m, fakeParser{role: "STAFF"})

	w := doReq(t, r, "POST", "/api/v1/products", `{"name":"Widget","sku":"W1"}`, "tok")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateUnauthenticated(t *testing.T) {
	m := new(mockRepo)
	r := setupProductEngine(m, fakeParser{err: sharederr.ErrUnauthorized})

	w := doReq(t, r, "POST", "/api/v1/products", `{"name":"Widget","sku":"W1"}`, "tok")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateValidation(t *testing.T) {
	m := new(mockRepo)
	r := setupProductEngine(m, fakeParser{role: "ADMIN"})

	w := doReq(t, r, "POST", "/api/v1/products", `{"name":"","sku":""}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAdminOK(t *testing.T) {
	m := new(mockRepo)
	catID := uuid.New()
	id := uuid.New()
	m.On("SKUExists", "W1", uuid.Nil).Return(false, nil)
	m.On("Create", mock.MatchedBy(func(p *Product) bool { return p.Name == "Widget" && p.CategoryID == catID })).
		Return(nil).Run(func(args mock.Arguments) {
		args.Get(0).(*Product).ID = id
	})

	r := setupProductEngine(m, fakeParser{role: "ADMIN"})
	body := `{"name":"Widget","sku":"W1","price":10,"category_id":"` + catID.String() + `"}`
	w := doReq(t, r, "POST", "/api/v1/products", body, "tok")

	assert.Equal(t, http.StatusCreated, w.Code)
	got := decodeProduct(t, w)
	assert.Equal(t, "Widget", got["data"].(map[string]any)["name"])
}

func TestHandlerCreateDuplicateSKUConflict(t *testing.T) {
	m := new(mockRepo)
	catID := uuid.New()
	m.On("SKUExists", "W1", uuid.Nil).Return(true, nil)

	r := setupProductEngine(m, fakeParser{role: "ADMIN"})
	body := `{"name":"Widget","sku":"W1","price":10,"category_id":"` + catID.String() + `"}`
	w := doReq(t, r, "POST", "/api/v1/products", body, "tok")

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestGetPublic(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", id).Return(&Product{ID: id, Name: "Widget"}, nil)

	r := setupProductEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/products/"+id.String(), "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Widget", decodeProduct(t, w)["data"].(map[string]any)["name"])
}

func TestHandlerGetNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", id).Return(nil, sharederr.ErrNotFound)

	r := setupProductEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/products/"+id.String(), "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateAdminOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	catID := uuid.New()
	m.On("SKUExists", "W2", id).Return(false, nil)
	m.On("Get", id).Return(&Product{ID: id, Name: "Old", CategoryID: catID}, nil)
	m.On("Update", mock.Anything).Return(nil)

	r := setupProductEngine(m, fakeParser{role: "ADMIN"})
	body := `{"name":"New","sku":"W2","price":20,"category_id":"` + catID.String() + `"}`
	w := doReq(t, r, "PUT", "/api/v1/products/"+id.String(), body, "tok")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateStaffForbidden(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()

	r := setupProductEngine(m, fakeParser{role: "STAFF"})
	w := doReq(t, r, "PUT", "/api/v1/products/"+id.String(), `{"name":"New"}`, "tok")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandlerUpdateNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	catID := uuid.New()
	m.On("SKUExists", "W3", id).Return(false, nil)
	m.On("Get", id).Return(nil, sharederr.ErrNotFound)

	r := setupProductEngine(m, fakeParser{role: "ADMIN"})
	body := `{"name":"New","sku":"W3","category_id":"` + catID.String() + `"}`
	w := doReq(t, r, "PUT", "/api/v1/products/"+id.String(), body, "tok")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListInvalidCategoryID(t *testing.T) {
	m := new(mockRepo)
	r := setupProductEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/products?category_id=not-a-uuid", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListArchivedFilter(t *testing.T) {
	m := new(mockRepo)
	m.On("List", mock.MatchedBy(func(q ListQuery) bool {
		return q.IsArchived != nil && *q.IsArchived
	})).Return([]*Product{}, int64(0), nil)

	r := setupProductEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/products?is_archived=true", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteAdminOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Delete", id).Return(nil)

	r := setupProductEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "DELETE", "/api/v1/products/"+id.String(), "", "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExportCSV(t *testing.T) {
	m := new(mockRepo)
	prods := []*Product{{ID: uuid.New(), Name: "Widget", SKU: "W1"}}
	m.On("List", mock.Anything).Return(prods, int64(1), nil)

	r := setupProductEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/products/export", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=")
	assert.Contains(t, w.Body.String(), "id,name,sku")
}
