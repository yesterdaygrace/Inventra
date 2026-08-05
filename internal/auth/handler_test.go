package auth

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
	"golang.org/x/crypto/bcrypt"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/validator"
)

func setupAuthEngine(repo *mockRepo) (*gin.Engine, *Service, *TokenManager) {
	gin.SetMode(gin.TestMode)
	tm := newTestManager()
	svc := NewService(repo, tm, bcrypt.DefaultCost)
	h := NewHandler(svc, validator.New())
	r := gin.New()
	group := r.Group("/api/v1")
	RegisterRoutes(group, h, tm)
	return r, svc, tm
}

func doJSON(t *testing.T, r *gin.Engine, method, path, body string, token string) *httptest.ResponseRecorder {
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

func TestRegisterLoginMeRoundtrip(t *testing.T) {
	uid := uuid.New()
	user := &User{ID: uid, Name: "Ada", Email: "ada@example.com", RoleID: staffRoleID, IsActive: true}
	loginUser := &User{
		ID: uid, Name: "Ada", Email: "ada@example.com",
		PasswordHash: hashedPassword("password123"), RoleID: staffRoleID, IsActive: true,
	}
	repo := &mockRepo{}
	repo.On("FindUserByEmail", "ada@example.com").Return(nil, sharederr.ErrNotFound).Once()
	repo.On("FindUserByEmail", "ada@example.com").Return(loginUser, nil).Once()
	repo.On("FindRoleByName", "STAFF").Return(staffRole, nil)
	repo.On("CreateUser", mock.AnythingOfType("*auth.User")).Return(nil).Run(func(args mock.Arguments) {
		args.Get(0).(*User).ID = uid
	})
	repo.On("FindUserByID", uid).Return(user, nil)
	repo.On("FindRoleByID", staffRoleID).Return(staffRole, nil)
	repo.On("CreateRefreshToken", mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything).Return(nil)

	r, _, _ := setupAuthEngine(repo)

	// register
	w := doJSON(t, r, "POST", "/api/v1/auth/register",
		`{"name":"Ada","email":"ada@example.com","password":"password123"}`, "")
	assert.Equal(t, http.StatusCreated, w.Code)

	// login
	w = doJSON(t, r, "POST", "/api/v1/auth/login",
		`{"email":"ada@example.com","password":"password123"}`, "")
	assert.Equal(t, http.StatusOK, w.Code)
	loginBody := decode(t, w)
	assert.True(t, loginBody["success"].(bool), "login should succeed")
	data := loginBody["data"].(map[string]any)
	accessToken := data["access_token"].(string)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, data["refresh_token"])
	assert.Equal(t, "Bearer", data["token_type"])

	// me with access token
	w = doJSON(t, r, "GET", "/api/v1/auth/me", "", accessToken)
	assert.Equal(t, http.StatusOK, w.Code)
	meBody := decode(t, w)
	assert.True(t, meBody["success"].(bool))
	meData := meBody["data"].(map[string]any)
	assert.Equal(t, "ada@example.com", meData["email"])
	assert.Equal(t, "STAFF", meData["role"])

	repo.AssertExpectations(t)
}

func TestProtectedRouteRejectsMissingToken(t *testing.T) {
	repo := &mockRepo{}
	r, _, _ := setupAuthEngine(repo)

	w := doJSON(t, r, "GET", "/api/v1/auth/me", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := decode(t, w)
	assert.Equal(t, false, body["success"])
}

func TestProtectedRouteRejectsInvalidToken(t *testing.T) {
	repo := &mockRepo{}
	r, _, _ := setupAuthEngine(repo)

	w := doJSON(t, r, "GET", "/api/v1/auth/me", "", "not-a-valid-token")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegisterValidationError(t *testing.T) {
	repo := &mockRepo{}
	r, _, _ := setupAuthEngine(repo)

	w := doJSON(t, r, "POST", "/api/v1/auth/register",
		`{"name":"","email":"bad","password":"short"}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
