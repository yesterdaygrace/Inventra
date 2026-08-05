// Package router builds the Gin engine with shared middleware and the
// healthcheck endpoint.
package router

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/config"
	"inventory/internal/shared/middleware"
)

// New constructs the base Gin engine with CORS, secure headers, request
// IDs, and a healthcheck route. Module routes are registered by callers.
func New(cfg *config.Config) *gin.Engine {
	r := gin.Default()
	r.Use(middleware.CORS(cfg.CORSOrigins))
	r.Use(middleware.SecureHeaders())
	r.Use(middleware.RequestID())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	return r
}
