// Package middleware provides cross-cutting Gin middleware: CORS,
// secure headers, and request-ID propagation.
package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDKey is the context key under which the request ID is stored.
const RequestIDKey = "request_id"

// CORS builds a CORS middleware honoring the configured origins.
func CORS(origins []string) gin.HandlerFunc {
	cfg := cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	if len(origins) == 0 {
		cfg.AllowOrigins = []string{"http://localhost:5173"}
	}
	return cors.New(cfg)
}

// SecureHeaders sets security-related response headers.
func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// RequestID ensures every request carries an X-Request-ID, echoing an
// inbound value or generating a UUID, and stores it in the context.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Header("X-Request-ID", rid)
		c.Set(RequestIDKey, rid)
		c.Next()
	}
}
