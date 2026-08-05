package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSecureHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	SecureHeaders()(c)

	if c.Writer.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("X-Content-Type-Options should be nosniff")
	}
	if c.Writer.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options should be DENY")
	}
	if c.Writer.Header().Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy should be set")
	}
}

func TestRequestIDAddsHeaderAndContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	RequestID()(c)

	rid := c.Writer.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatal("X-Request-ID header not set")
	}
	fromCtx, ok := c.Get(RequestIDKey)
	if !ok || fromCtx != rid {
		t.Errorf("request id not in context: got %v (ok=%v), want %q", fromCtx, ok, rid)
	}
}

func TestRequestIDEchoesIncoming(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-supplied-123")
	c.Request = req

	RequestID()(c)

	if got := c.Writer.Header().Get("X-Request-ID"); got != "client-supplied-123" {
		t.Errorf("X-Request-ID = %q, want echo of client value", got)
	}
}
