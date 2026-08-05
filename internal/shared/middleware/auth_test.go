package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeParser is a ClaimsParser stub for tests.
type fakeParser struct {
	uid  uuid.UUID
	role string
	err  error
}

func (f *fakeParser) ParseAccessToken(raw string) (uuid.UUID, string, error) {
	if f.err != nil {
		return uuid.Nil, "", f.err
	}
	return f.uid, f.role, nil
}

func setupProtectedEngine(p ClaimsParser, roles ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	grp := r.Group("/protected")
	grp.Use(Auth(p))
	if len(roles) > 0 {
		grp.Use(RoleRequired(roles...))
	}
	grp.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": UserIDFromContext(c).String(), "role": RoleFromContext(c)})
	})
	return r
}

func TestAuthAcceptsValidToken(t *testing.T) {
	uid := uuid.New()
	r := setupProtectedEngine(&fakeParser{uid: uid, role: "ADMIN"})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), uid.String())
	require.Contains(t, w.Body.String(), "ADMIN")
}

func TestAuthRejectsMissingHeader(t *testing.T) {
	r := setupProtectedEngine(&fakeParser{uid: uuid.New(), role: "ADMIN"})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRejectsMalformedHeader(t *testing.T) {
	r := setupProtectedEngine(&fakeParser{uid: uuid.New(), role: "ADMIN"})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Token valid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRejectsParseFailure(t *testing.T) {
	r := setupProtectedEngine(&fakeParser{err: assert.AnError})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer tampered")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRoleRequiredAllowsAdmin(t *testing.T) {
	r := setupProtectedEngine(&fakeParser{uid: uuid.New(), role: "ADMIN"}, "ADMIN")

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoleRequiredBlocksStaffFromAdminRoute(t *testing.T) {
	r := setupProtectedEngine(&fakeParser{uid: uuid.New(), role: "STAFF"}, "ADMIN")

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRoleRequiredAllowsEitherRole(t *testing.T) {
	r := setupProtectedEngine(&fakeParser{uid: uuid.New(), role: "STAFF"}, "ADMIN", "STAFF")

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
