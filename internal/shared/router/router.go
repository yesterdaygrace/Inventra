// Package router builds the Gin engine with shared middleware and the
// healthcheck endpoint.
package router

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"inventory/internal/shared/config"
	"inventory/internal/shared/middleware"
)

// New constructs the base Gin engine with CORS, secure headers, request
// IDs, a Zap request logger, and healthcheck/readiness routes. Module
// routes are registered by callers. A nil logger falls back to a no-op.
// db may be nil: /ready then always reports unavailable.
func New(cfg *config.Config, log *zap.Logger, db *sql.DB) *gin.Engine {
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
	r.GET("/ready", func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service": "inventra",
			"version": "1.0.0",
			"docs":    "/swagger/index.html",
			"health":  "/healthz",
			"ready":   "/ready",
			"api":     "/api/v1",
		})
	})
	return r
}
