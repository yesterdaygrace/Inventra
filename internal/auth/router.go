// Route registration for the auth module.
package auth

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RateLimits carries the per-minute request budgets for the public auth
// routes. Zero values fall back to the middleware's own safe minimum.
type RateLimits struct {
	LoginRPM    int
	RefreshRPM  int
	RegisterRPM int
	DemoRPM     int
}

// RegisterRoutes wires all auth endpoints on the provided group. When
// demoMode is true it additionally exposes the public demo auto-login
// endpoint POST /auth/demo. Public (unauthenticated) routes are rate
// limited per client IP using the provided budgets.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, tm *TokenManager, demoMode bool, limits RateLimits) {
	parser := NewTokenParser(tm)
	authGroup := group.Group("/auth")

	authGroup.POST("/register", middleware.RateLimit(limits.RegisterRPM), h.Register)
	authGroup.POST("/login", middleware.RateLimit(limits.LoginRPM), h.Login)
	authGroup.POST("/refresh", middleware.RateLimit(limits.RefreshRPM), h.Refresh)
	if demoMode {
		authGroup.POST("/demo", middleware.RateLimit(limits.DemoRPM), h.DemoLogin)
	}

	protected := authGroup.Group("")
	protected.Use(middleware.Auth(parser))
	{
		protected.POST("/logout", h.Logout)
		protected.POST("/change-password", h.ChangePassword)
		protected.PUT("/profile", h.UpdateProfile)
		protected.GET("/me", h.Me)
	}
}
