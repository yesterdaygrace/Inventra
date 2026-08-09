package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"inventory/internal/shared/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealthz(t *testing.T) {
	cfg := &config.Config{}
	r := New(cfg, zap.NewNop())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
}

func TestHealthzHasRequestID(t *testing.T) {
	cfg := &config.Config{}
	r := New(cfg, zap.NewNop())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header should be present")
	}
	if w.Header().Get("X-Content-Type-Options") == "" {
		t.Error("secure headers middleware should run")
	}
}
