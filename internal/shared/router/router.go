// Package router builds the Gin engine with shared middleware and the
// healthcheck endpoint.
package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"inventory/internal/shared/config"
	"inventory/internal/shared/middleware"
)

// New constructs the base Gin engine with CORS, secure headers, request
// IDs, a Zap request logger, and a healthcheck route. Module routes are
// registered by callers. A nil logger falls back to a no-op (no logging).
func New(cfg *config.Config, log *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	if log != nil {
		r.Use(middleware.RequestLogger(log))
	}
	r.Use(middleware.CORS(cfg.CORSOrigins))
	r.Use(middleware.SecureHeaders())
	r.Use(middleware.RequestID())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service": "inventra",
			"version": "1.0.0",
			"docs":    "/swagger/index.html",
			"health":  "/healthz",
			"api":     "/api/v1",
		})
	})
	return r
}
