package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// responseWriter captures the status code written by the handler so the
// request logger can report it.
type responseWriter struct {
	gin.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// RequestLogger emits a structured Zap log line per request with method,
// path, status, latency, client IP, and request ID.
func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: c.Writer}
		c.Writer = rw
		c.Next()
		log.Info("http request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", rw.status),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
			zap.String("request_id", c.GetString(RequestIDKey)),
		)
	}
}