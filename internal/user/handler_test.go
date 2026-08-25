package user

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

func (p fakeParser) ParseAccessToken(raw string) (uuid.UUID, string, []string, error) {
	if p.perms != nil {
		return p.userID, p.role, p.perms, p.err
	}
	return p.userID, p.role, auth.PermissionSetForRole(p.role), p.err
}

func setupUserEngine(repo Repository, parser middleware.ClaimsParser) *gin.Engine {
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

func decodeUser(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestListRequiresAuth(t *testing.T) {
	m := new(mockRepo)
	r := setupUserEngine(m, fakeParser{err: sharederr.ErrUnauthorized})

	w := doReq(t, r, "GET", "/api/v1/users", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListAdminOK(t *testing.T) {
	m := new(mockRepo)
	adminID := uuid.New()
	users := []*User{
		{ID: uuid.New(), Name: "Admin", Email: "admin@example.com", Role: Role{Name: "ADMIN"}, IsActive: true},
	}
	m.On("List", mock.Anything, mock.MatchedBy(func(q Query) bool { return q.Page == 1 })).
		Return(users, int64(1), nil)

	r := setupUserEngine(m, fakeParser{userID: adminID, role: "ADMIN"})
	w := doReq(t, r, "GET", "/api/v1/users?page=1&per_page=20", "", "tok")

	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeUser(t, w)
	assert.Contains(t, body, "data")
	assert.NotNil(t, body["meta"])
}

func TestListRejectsStaff(t *testing.T) {
	m := new(mockRepo)
	r := setupUserEngine(m, fakeParser{userID: uuid.New(), role: "STAFF"})

	w := doReq(t, r, "GET", "/api/v1/users", "", "tok")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListBadIsActiveFilter(t *testing.T) {
	m := new(mockRepo)
	r := setupUserEngine(m, fakeParser{userID: uuid.New(), role: "ADMIN"})

	w := doReq(t, r, "GET", "/api/v1/users?is_active=banana", "", "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetUser(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, Name: "Alice", Email: "alice@example.com", Role: Role{Name: "STAFF"}, IsActive: true}
	m.On("FindByID", mock.Anything, id).Return(u, nil)

	r := setupUserEngine(m, fakeParser{userID: uuid.New(), role: "ADMIN"})
	w := doReq(t, r, "GET", "/api/v1/users/"+id.String(), "", "tok")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Alice", decodeUser(t, w)["data"].(map[string]any)["name"])
}

func TestGetUserNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("FindByID", mock.Anything, id).Return(nil, sharederr.ErrNotFound)

	r := setupUserEngine(m, &fakeParser{userID: uuid.New(), role: "ADMIN"})
	w := doReq(t, r, "GET", "/api/v1/users/"+id.String(), "", "tok")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateUser(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, Email: "a@example.com", Name: "Old"}
	m.On("FindByID", mock.Anything, id).Return(u, nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	r := setupUserEngine(m, &fakeParser{userID: uuid.New(), role: "ADMIN"})
	w := doReq(t, r, "PUT", "/api/v1/users/"+id.String(), `{"name":"New"}`, "tok")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateUserEmptyBodyRejected(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	r := setupUserEngine(m, &fakeParser{userID: uuid.New(), role: "ADMIN"})

	w := doReq(t, r, "PUT", "/api/v1/users/"+id.String(), `{}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteUserRejectsSelf(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("FindByID", mock.Anything, id).Return(&User{ID: id}, nil)

	r := setupUserEngine(m, &fakeParser{userID: id, role: "ADMIN"})
	w := doReq(t, r, "DELETE", "/api/v1/users/"+id.String(), "", "tok")
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAssignRoleHandler(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	role := &Role{ID: uuid.New(), Name: "ADMIN"}
	u := &User{ID: id, Name: "Alice"}
	m.On("FindRoleByName", mock.Anything, "ADMIN").Return(role, nil)
	m.On("FindByID", mock.Anything, id).Return(u, nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	r := setupUserEngine(m, &fakeParser{userID: uuid.New(), role: "ADMIN"})
	w := doReq(t, r, "PUT", "/api/v1/users/"+id.String()+"/role", `{"role":"ADMIN"}`, "tok")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAssignRoleInvalidRole(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	r := setupUserEngine(m, &fakeParser{userID: uuid.New(), role: "ADMIN"})

	w := doReq(t, r, "PUT", "/api/v1/users/"+id.String()+"/role", `{"role":"SUPERUSER"}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
