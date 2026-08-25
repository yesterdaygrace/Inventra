package category

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"inventory/internal/auth"
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
	m.On("List", mock.Anything, mock.MatchedBy(func(q ListQuery) bool { return q.Search == "elec" })).
		Return(cats, int64(1), nil)

	r := setupCategoryEngine(m, fakeParser{role: "STAFF"})
	w := doReq(t, r, "GET", "/api/v1/categories?name=elec", "", "tok")

	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeCategory(t, w)
	assert.Contains(t, body, "data")
	assert.NotNil(t, body["meta"])
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
	m.On("Create", mock.Anything, mock.MatchedBy(func(c *Category) bool { return c.Name == "Books" })).
		Return(nil).Run(func(args mock.Arguments) {
		args.Get(1).(*Category).ID = cat.ID
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
	m.On("Get", mock.Anything, id).Return(&Category{ID: id, Name: "Electronics"}, nil)

	r := setupCategoryEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/categories/"+id.String(), "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Electronics", decodeCategory(t, w)["data"].(map[string]any)["name"])
}

func TestGetNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(nil, sharederr.ErrNotFound)

	r := setupCategoryEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/categories/"+id.String(), "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateAdminOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(&Category{ID: id, Name: "Old"}, nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "PUT", "/api/v1/categories/"+id.String(), `{"name":"New"}`, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateDescriptionOnlyPartial(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(&Category{ID: id, Name: "Keep"}, nil)
	m.On("Update", mock.Anything, mock.MatchedBy(func(c *Category) bool {
		return c.Name == "Keep" && c.Description != nil && *c.Description == "new"
	})).Return(nil)

	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "PUT", "/api/v1/categories/"+id.String(), `{"description":"new"}`, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateEmptyBodyRejected(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()

	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "PUT", "/api/v1/categories/"+id.String(), `{}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	m.AssertNotCalled(t, "Get", mock.Anything)
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
	m.On("CountProductsFor", mock.Anything, id).Return(int64(1), nil)

	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "DELETE", "/api/v1/categories/"+id.String(), "", "tok")
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandlerDeleteOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("CountProductsFor", mock.Anything, id).Return(int64(0), nil)
	m.On("Delete", mock.Anything, id).Return(nil)

	r := setupCategoryEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "DELETE", "/api/v1/categories/"+id.String(), "", "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExportCSV(t *testing.T) {
	m := new(mockRepo)
	cats := []*Category{{ID: uuid.New(), Name: "Books"}}
	m.On("List", mock.Anything, mock.Anything).Return(cats, int64(1), nil)

	r := setupCategoryEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/categories/export", "", "")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=")
	assert.Contains(t, w.Body.String(), "id,name,description,created_at")
}

// TestExportPaginatesBeyondCap guards the category export against the
// per-page cap silently dropping rows beyond the first 100 categories.
func TestExportPaginatesBeyondCap(t *testing.T) {
	m := new(mockRepo)
	const total = 120
	page1 := make([]*Category, 100)
	for i := range page1 {
		page1[i] = &Category{ID: uuid.New(), Name: fmt.Sprintf("Cat%03d", i)}
	}
	page2 := make([]*Category, total-100)
	for i := range page2 {
		page2[i] = &Category{ID: uuid.New(), Name: fmt.Sprintf("More%02d", i)}
	}
	m.On("List", mock.Anything, mock.MatchedBy(func(q ListQuery) bool { return q.Page == 1 && q.PerPage == 100 })).
		Return(page1, int64(total), nil)
	m.On("List", mock.Anything, mock.MatchedBy(func(q ListQuery) bool { return q.Page == 2 && q.PerPage == 100 })).
		Return(page2, int64(total), nil)

	r := setupCategoryEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/categories/export", "", "")

	require.Equal(t, http.StatusOK, w.Code)
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	assert.Len(t, lines, total+1, "header + every row must be exported across pages")
}
