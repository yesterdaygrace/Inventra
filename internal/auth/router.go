// Route registration for the auth module.
package auth

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires all auth endpoints on the provided group. When
// demoMode is true it additionally exposes the public demo auto-login
// endpoint POST /auth/demo.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, tm *TokenManager, demoMode bool) {
	parser := NewTokenParser(tm)
	authGroup := group.Group("/auth")

	authGroup.POST("/register", h.Register)
	authGroup.POST("/login", h.Login)
	authGroup.POST("/refresh", h.Refresh)
	if demoMode {
		authGroup.POST("/demo", h.DemoLogin)
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
