package category

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

func setupCategoryEngine(repo Repository, parser middleware.ClaimsParser) *gin.Engine {
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

func decodeCategory(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestListPublicPaginated(t *testing.T) {
	m := new(mockRepo)
	cats := []*Category{{Name: "Electronics"}}
	m.On("List", mock.MatchedBy(func(q ListQuery) bool { return q.Search == "elec" })).
		Return(cats, int64(1), nil)

	r := setupCategoryEngine(m, fakeParser{role: "STAFF"})
	w := doReq(t, r, "GET", "/api/v1/categories?name=elec", "", "tok")

	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeCategory(t, w)
	assert.True(t, body["success"].(bool))
	assert.NotNil(t, body["pagination"])
}

func TestCreateRequiresAdmin(t *testing.T) {
	m := new(mockRepo)
	r := setupCategoryEngine(m, fakeParser{role: "STAFF"})

	w := doReq(t, r, "POST", "/api/v1/categories", `{"name":"Books"}`, "tok")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateUnauthenticated(t *testing.T) {
	m := new(mockRepo)
	r := setupCategoryEngine(m, fakeParser{err: sharederr.ErrUnauthorized})

	w := doReq(t, r, "POST", "/api/v1/categories", `{"name":"Books"}`, "tok")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateAdminOK(t *testing.T) {
	m := new(mockRepo)
	cat := &Category{ID: uuid.New(), Name: "Books"}
	m.On("Create", mock.MatchedBy(func(c *Category) bool { return c.Name == "Books" })).
		Return(nil).Run(func(args mock.Arguments) {
		args.Get(0).(*Category).ID = cat.ID
	})

	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "POST", "/api/v1/categories", `{"name":"Books"}`, "tok")

	assert.Equal(t, http.StatusCreated, w.Code)
	body := decodeCategory(t, w)
	assert.Equal(t, "Books", body["data"].(map[string]any)["name"])
}

func TestCreateValidation(t *testing.T) {
	m := new(mockRepo)
	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})

	w := doReq(t, r, "POST", "/api/v1/categories", `{"name":""}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerGet(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", id).Return(&Category{ID: id, Name: "Electronics"}, nil)

	r := setupCategoryEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/categories/"+id.String(), "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Electronics", decodeCategory(t, w)["data"].(map[string]any)["name"])
}

func TestGetNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", id).Return(nil, sharederr.ErrNotFound)

	r := setupCategoryEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/categories/"+id.String(), "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateAdminOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", id).Return(&Category{ID: id, Name: "Old"}, nil)
	m.On("Update", mock.Anything).Return(nil)

	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "PUT", "/api/v1/categories/"+id.String(), `{"name":"New"}`, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateStaffForbidden(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()

	r := setupCategoryEngine(m, fakeParser{role: "STAFF"})
	w := doReq(t, r, "PUT", "/api/v1/categories/"+id.String(), `{"name":"New"}`, "tok")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteInUseConflict(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("CountProductsFor", id).Return(int64(1), nil)

	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "DELETE", "/api/v1/categories/"+id.String(), "", "tok")
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandlerDeleteOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("CountProductsFor", id).Return(int64(0), nil)
	m.On("Delete", id).Return(nil)

	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "DELETE", "/api/v1/categories/"+id.String(), "", "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExportCSV(t *testing.T) {
	m := new(mockRepo)
	cats := []*Category{{ID: uuid.New(), Name: "Books"}}
	m.On("List", mock.Anything).Return(cats, int64(1), nil)

	r := setupCategoryEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/categories/export", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=")
	assert.Contains(t, w.Body.String(), "id,name,description,created_at")
}
