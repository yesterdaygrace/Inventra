package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"inventory/internal/shared/audit"
	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/validator"
)

func setupAuthEngine(repo *mockRepo) (*gin.Engine, *Service, *TokenManager) {
	return setupAuthEngineMode(repo, false)
}

// setupAuthEngineMode builds the engine with demo auto-login enabled or
// disabled according to demoMode.
func setupAuthEngineMode(repo *mockRepo, demoMode bool) (*gin.Engine, *Service, *TokenManager) {
	gin.SetMode(gin.TestMode)
	tm := newTestManager()
	svc := NewService(repo, tm, bcrypt.DefaultCost)
	h := NewHandler(svc, validator.New())
	r := gin.New()
	group := r.Group("/api/v1")
	RegisterRoutes(group, h, tm, demoMode)
	return r, svc, tm
}

// setupAuthEngineWith builds the engine over an explicit TokenManager so
// callers can pre-compute refresh-token hashes with the same manager the
// service will use to verify them.
func setupAuthEngineWith(tm *TokenManager, repo *mockRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := NewService(repo, tm, bcrypt.DefaultCost)
	h := NewHandler(svc, validator.New())
	r := gin.New()
	group := r.Group("/api/v1")
	RegisterRoutes(group, h, tm, false)
	return r
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
	repo.On("FindUserByEmail", mock.Anything, "ada@example.com").Return(nil, sharederr.ErrNotFound).Once()
	repo.On("FindUserByEmail", mock.Anything, "ada@example.com").Return(loginUser, nil).Once()
	repo.On("FindRoleByName", mock.Anything, "STAFF").Return(staffRole, nil)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*auth.User")).Return(nil).Run(func(args mock.Arguments) {
		args.Get(1).(*User).ID = uid
	})
	repo.On("FindUserByID", mock.Anything, uid).Return(user, nil)
	repo.On("FindRoleByID", mock.Anything, staffRoleID).Return(staffRole, nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

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

func TestRegisterLoginValidationErrors(t *testing.T) {
	repo := &mockRepo{}
	r, _, _ := setupAuthEngine(repo)

	w := doJSON(t, r, "POST", "/api/v1/auth/login",
		`{"email":"not-an-email","password":""}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRefreshValidationError(t *testing.T) {
	repo := &mockRepo{}
	r, _, _ := setupAuthEngine(repo)
	w := doJSON(t, r, "POST", "/api/v1/auth/refresh", `{"refresh_token":""}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// accessTokenFor signs a valid access token for the given user so protected
// routes can be exercised with the shared test manager.
func accessTokenFor(t *testing.T, tm *TokenManager, uid uuid.UUID, role string) string {
	t.Helper()
	raw, err := tm.SignAccessToken(uid, role)
	require.NoError(t, err)
	return raw
}

func TestHandler_RefreshRoundtrip(t *testing.T) {
	uid := uuid.New()
	user := &User{ID: uid, Name: "Ada", Email: "ada@refresh.test", RoleID: staffRoleID, IsActive: true}
	expires := time.Now().Add(7 * 24 * time.Hour)
	raw := "refreshtokraw"
	tm := newTestManager()
	hash := tm.HashRefreshToken(raw)
	rt := &RefreshToken{ID: uuid.New(), UserID: uid, TokenHash: hash, ExpiresAt: expires}

	repo := &mockRepo{}
	repo.On("FindRefreshTokenByHash", mock.Anything, hash).Return(rt, nil)
	repo.On("FindUserByID", mock.Anything, uid).Return(user, nil)
	repo.On("UpdateRefreshToken", mock.Anything, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("FindRoleByID", mock.Anything, staffRoleID).Return(staffRole, nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

	r := setupAuthEngineWith(tm, repo)
	w := doJSON(t, r, "POST", "/api/v1/auth/refresh", `{"refresh_token":"refreshtokraw"}`, "")
	assert.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.NotEmpty(t, data["access_token"])
	assert.NotEmpty(t, data["refresh_token"])

	repo.AssertExpectations(t)
}

func TestHandler_RefreshRejectsUnknownToken(t *testing.T) {
	tm := newTestManager()
	hash := tm.HashRefreshToken("unknownraw")
	repo := &mockRepo{}
	repo.On("FindRefreshTokenByHash", mock.Anything, hash).Return(nil, sharederr.ErrNotFound)
	r, _, _ := setupAuthEngine(repo)
	w := doJSON(t, r, "POST", "/api/v1/auth/refresh", `{"refresh_token":"unknownraw"}`, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_LogoutRevokesToken(t *testing.T) {
	uid := uuid.New()
	raw := "logoutraw"
	tm := newTestManager()
	hash := tm.HashRefreshToken(raw)
	rt := &RefreshToken{ID: uuid.New(), UserID: uid, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}

	repo := &mockRepo{}
	repo.On("FindRefreshTokenByHash", mock.Anything, hash).Return(rt, nil)
	repo.On("UpdateRefreshToken", mock.Anything, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

	r := setupAuthEngineWith(tm, repo)
	token := accessTokenFor(t, tm, uid, "STAFF")
	w := doJSON(t, r, "POST", "/api/v1/auth/logout", `{"refresh_token":"logoutraw"}`, token)
	assert.Equal(t, http.StatusOK, w.Code)

	repo.AssertExpectations(t)
}

func TestHandler_LogoutIdempotentWhenTokenUnknown(t *testing.T) {
	tm := newTestManager()
	hash := tm.HashRefreshToken("missingraw")
	repo := &mockRepo{}
	repo.On("FindRefreshTokenByHash", mock.Anything, hash).Return(nil, sharederr.ErrNotFound)

	r, _, tm := setupAuthEngine(repo)
	token := accessTokenFor(t, tm, uuid.New(), "STAFF")
	w := doJSON(t, r, "POST", "/api/v1/auth/logout", `{"refresh_token":"missingraw"}`, token)
	assert.Equal(t, http.StatusOK, w.Code)

	repo.AssertExpectations(t)
}

func TestHandler_ChangePassword(t *testing.T) {
	uid := uuid.New()
	user := &User{
		ID: uid, Name: "Ada", Email: "ada@pw.test",
		PasswordHash: hashedPassword("oldpass123"), RoleID: staffRoleID, IsActive: true,
	}

	repo := &mockRepo{}
	repo.On("FindUserByID", mock.Anything, uid).Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*auth.User")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

	r, _, tm := setupAuthEngine(repo)
	token := accessTokenFor(t, tm, uid, "STAFF")
	w := doJSON(t, r, "POST", "/api/v1/auth/change-password",
		`{"old_password":"oldpass123","new_password":"newpass456"}`, token)
	assert.Equal(t, http.StatusOK, w.Code)

	repo.AssertExpectations(t)
}

func TestHandler_ChangePasswordWrongOld(t *testing.T) {
	uid := uuid.New()
	user := &User{
		ID: uid, Name: "Ada", Email: "ada@pw.test",
		PasswordHash: hashedPassword("oldpass123"), RoleID: staffRoleID, IsActive: true,
	}
	repo := &mockRepo{}
	repo.On("FindUserByID", mock.Anything, uid).Return(user, nil)

	r, _, tm := setupAuthEngine(repo)
	token := accessTokenFor(t, tm, uid, "STAFF")
	w := doJSON(t, r, "POST", "/api/v1/auth/change-password",
		`{"old_password":"wrongpass","new_password":"newpass456"}`, token)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_UpdateProfile(t *testing.T) {
	uid := uuid.New()
	user := &User{ID: uid, Name: "Ada", Email: "ada@profile.test", RoleID: staffRoleID, IsActive: true}
	updated := &User{ID: uid, Name: "Ada Lovelace", Email: "ada@profile.test", RoleID: staffRoleID, IsActive: true}

	repo := &mockRepo{}
	repo.On("FindUserByID", mock.Anything, uid).Return(user, nil)
	repo.On("FindUserByEmail", mock.Anything, "new@profile.test").Return(nil, sharederr.ErrNotFound)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*auth.User")).Return(nil).Run(func(args mock.Arguments) {
		*args.Get(1).(*User) = *updated
	})
	repo.On("FindRoleByID", mock.Anything, staffRoleID).Return(staffRole, nil)
	repo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

	r, _, tm := setupAuthEngine(repo)
	token := accessTokenFor(t, tm, uid, "STAFF")
	w := doJSON(t, r, "PUT", "/api/v1/auth/profile",
		`{"name":"Ada Lovelace","email":"new@profile.test"}`, token)
	assert.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	assert.True(t, body["success"].(bool))

	repo.AssertExpectations(t)
}

func TestHandler_ChangePasswordValidationError(t *testing.T) {
	repo := &mockRepo{}
	r, _, tm := setupAuthEngine(repo)
	token := accessTokenFor(t, tm, uuid.New(), "STAFF")
	w := doJSON(t, r, "POST", "/api/v1/auth/change-password",
		`{"old_password":"","new_password":"short"}`, token)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UpdateProfileValidationError(t *testing.T) {
	repo := &mockRepo{}
	r, _, tm := setupAuthEngine(repo)
	token := accessTokenFor(t, tm, uuid.New(), "STAFF")
	w := doJSON(t, r, "PUT", "/api/v1/auth/profile", `{"name":"x"}`, token)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_MeWhenUserMissing(t *testing.T) {
	uid := uuid.New()
	repo := &mockRepo{}
	repo.On("FindUserByID", mock.Anything, uid).Return(nil, sharederr.ErrNotFound)
	r, _, tm := setupAuthEngine(repo)
	token := accessTokenFor(t, tm, uid, "STAFF")
	w := doJSON(t, r, "GET", "/api/v1/auth/me", "", token)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_LoginInactiveUser(t *testing.T) {
	inactive := &User{
		ID: uuid.New(), Name: "Nope", Email: "inactive@login.test",
		PasswordHash: hashedPassword("password123"), RoleID: staffRoleID, IsActive: false,
	}
	repo := &mockRepo{}
	repo.On("FindUserByEmail", mock.Anything, "inactive@login.test").Return(inactive, nil)
	r, _, _ := setupAuthEngine(repo)
	w := doJSON(t, r, "POST", "/api/v1/auth/login",
		`{"email":"inactive@login.test","password":"password123"}`, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_LoginWrongPassword(t *testing.T) {
	user := &User{
		ID: uuid.New(), Name: "Ada", Email: "ada@wp.test",
		PasswordHash: hashedPassword("rightpass"), RoleID: staffRoleID, IsActive: true,
	}
	repo := &mockRepo{}
	repo.On("FindUserByEmail", mock.Anything, "ada@wp.test").Return(user, nil)
	r, _, _ := setupAuthEngine(repo)
	w := doJSON(t, r, "POST", "/api/v1/auth/login",
		`{"email":"ada@wp.test","password":"wrongpass"}`, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_ProtectedRouteRejectsMalformedJSON(t *testing.T) {
	repo := &mockRepo{}
	r, _, tm := setupAuthEngine(repo)
	token := accessTokenFor(t, tm, uuid.New(), "STAFF")
	w := doJSON(t, r, "POST", "/api/v1/auth/change-password", `{bad json`, token)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_SetAuditNilSafe(t *testing.T) {
	svc := NewService(&mockRepo{}, newTestManager(), bcrypt.DefaultCost)
	h := NewHandler(svc, validator.New())
	h.SetAudit(nil)
	h.SetAudit(&spyRecorder{})
}

// spyRecorder records audit events for the SetAudit wiring test.
type spyRecorder struct {
	called bool
}

func (s *spyRecorder) Record(audit.Entry) { s.called = true }

func TestHandler_RegisterRoleLookupError(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindUserByEmail", mock.Anything, mock.Anything).Return(nil, sharederr.ErrNotFound)
	repo.On("FindRoleByName", mock.Anything, "STAFF").Return(nil, sharederr.ErrNotFound)
	r, _, _ := setupAuthEngine(repo)
	w := doJSON(t, r, "POST", "/api/v1/auth/register",
		`{"name":"Ada","email":"ada@rl.test","password":"password123"}`, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_DemoLoginEnabled(t *testing.T) {
	uid := uuid.New()
	repo := &mockRepo{}
	repo.On("FindUserByEmail", mock.Anything, DemoEmail).Return(nil, sharederr.ErrNotFound)
	repo.On("FindRoleByName", mock.Anything, "STAFF").Return(staffRole, nil)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*auth.User")).Return(nil).Run(func(args mock.Arguments) {
		args.Get(1).(*User).ID = uid
	})
	repo.On("FindRoleByID", mock.Anything, staffRoleID).Return(staffRole, nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

	r, _, _ := setupAuthEngineMode(repo, true)
	w := doJSON(t, r, "POST", "/api/v1/auth/demo", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.NotEmpty(t, data["access_token"])
	assert.Equal(t, DemoEmail, data["user"].(map[string]any)["email"])
	repo.AssertExpectations(t)
}

func TestHandler_DemoLoginReusesExisting(t *testing.T) {
	uid := uuid.New()
	existing := &User{ID: uid, Email: DemoEmail, Name: "Demo User", RoleID: staffRoleID, IsActive: true}
	repo := &mockRepo{}
	repo.On("FindUserByEmail", mock.Anything, DemoEmail).Return(existing, nil)
	repo.On("FindRoleByID", mock.Anything, staffRoleID).Return(staffRole, nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)

	r, _, _ := setupAuthEngineMode(repo, true)
	w := doJSON(t, r, "POST", "/api/v1/auth/demo", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertNotCalled(t, "FindRoleByName")
	repo.AssertNumberOfCalls(t, "CreateUser", 0)
}

func TestHandler_DemoLoginDisabledReturnsNotFound(t *testing.T) {
	repo := &mockRepo{}
	r, _, _ := setupAuthEngine(repo)
	w := doJSON(t, r, "POST", "/api/v1/auth/demo", "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertNotCalled(t, "FindUserByEmail")
}

func TestHandler_DemoTokenAuthorizesProtectedRoute(t *testing.T) {
	uid := uuid.New()
	repo := &mockRepo{}
	repo.On("FindUserByEmail", mock.Anything, DemoEmail).Return(nil, sharederr.ErrNotFound)
	repo.On("FindRoleByName", mock.Anything, "STAFF").Return(staffRole, nil)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*auth.User")).Return(nil).Run(func(args mock.Arguments) {
		args.Get(1).(*User).ID = uid
	})
	repo.On("FindRoleByID", mock.Anything, staffRoleID).Return(staffRole, nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything, mock.Anything).Return(nil)
	repo.On("FindUserByID", mock.Anything, uid).Return(
		&User{ID: uid, Email: DemoEmail, RoleID: staffRoleID, IsActive: true},
		nil,
	)

	r, _, _ := setupAuthEngineMode(repo, true)
	w := doJSON(t, r, "POST", "/api/v1/auth/demo", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	data := decode(t, w)["data"].(map[string]any)
	accessToken := data["access_token"].(string)

	w = doJSON(t, r, "GET", "/api/v1/auth/me", "", accessToken)
	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}
