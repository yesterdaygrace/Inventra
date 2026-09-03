package audit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"inventory/internal/shared/middleware"
)

func TestEntryFromContextCapturesRequestFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uid := uuid.New()
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/op", middleware.Auth(&fakeParser{uid: uid, role: "ADMIN"}), func(c *gin.Context) {
		e := EntryFromContext(c, Entry{Action: "CREATE", EntityType: "product"})
		require.NotNil(t, e.UserID)
		assert.Equal(t, uid, *e.UserID)
		require.NotNil(t, e.UserAgent)
		assert.Equal(t, "qa/1.0", *e.UserAgent)
		require.NotNil(t, e.RequestID)
		assert.NotEmpty(t, *e.RequestID)
		require.NotNil(t, e.IP)
		assert.NotEmpty(t, *e.IP)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/op", nil)
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("User-Agent", "qa/1.0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEntryFromContextAnonymousLeavesUserNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/op", func(c *gin.Context) {
		e := EntryFromContext(c, Entry{Action: "LOGIN", EntityType: "user"})
		assert.Nil(t, e.UserID)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/op", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNopRecorderIgnoresEntry(t *testing.T) {
	var n Nop
	assert.NotPanics(t, func() {
		n.Record(Entry{Action: "CREATE", EntityType: "product"})
	})
}

type fakeParser struct {
	uid  uuid.UUID
	role string
	err  error
}

func (f *fakeParser) ParseAccessToken(raw string) (uuid.UUID, string, []string, error) {
	return f.uid, f.role, nil, f.err
}