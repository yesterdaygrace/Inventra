package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRateEngine(t *testing.T, rpm int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(rpm))
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func doRateReq(r *gin.Engine, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip + ":1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimitAllowsBurstWithinBudget(t *testing.T) {
	r := setupRateEngine(t, 3)
	for i := 0; i < 3; i++ {
		w := doRateReq(r, "10.0.0.1")
		assert.Equal(t, http.StatusOK, w.Code, "request %d within budget should pass", i+1)
	}
}

func TestRateLimitExceedsBudgetReturns429(t *testing.T) {
	r := setupRateEngine(t, 2)
	require.Equal(t, http.StatusOK, doRateReq(r, "10.0.0.1").Code)
	require.Equal(t, http.StatusOK, doRateReq(r, "10.0.0.1").Code)

	w := doRateReq(r, "10.0.0.1")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
}

func TestRateLimitIsPerIP(t *testing.T) {
	r := setupRateEngine(t, 1)
	require.Equal(t, http.StatusOK, doRateReq(r, "10.0.0.1").Code)
	// Second request from the same IP is limited...
	w := doRateReq(r, "10.0.0.1")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	// ...but a different IP has its own independent budget.
	other := doRateReq(r, "10.0.0.2")
	assert.Equal(t, http.StatusOK, other.Code)
}

func TestRateLimitResponseUsesStandardEnvelope(t *testing.T) {
	r := setupRateEngine(t, 0) // 0 -> clamps to the safe minimum (1)
	require.Equal(t, http.StatusOK, doRateReq(r, "10.0.0.1").Code)

	w := doRateReq(r, "10.0.0.1")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.JSONEq(t, `{"error": {"code": "RATE_LIMITED", "message": "rate limit exceeded"}}`, w.Body.String())
}
