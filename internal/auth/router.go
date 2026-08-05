// Route registration for the auth module.
package auth

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires all auth endpoints on the provided group.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, tm *TokenManager) {
	parser := NewTokenParser(tm)
	authGroup := group.Group("/auth")

	authGroup.POST("/register", h.Register)
	authGroup.POST("/login", h.Login)
	authGroup.POST("/refresh", h.Refresh)

	protected := authGroup.Group("")
	protected.Use(middleware.Auth(parser))
	{
		protected.POST("/logout", h.Logout)
		protected.POST("/change-password", h.ChangePassword)
		protected.PUT("/profile", h.UpdateProfile)
		protected.GET("/me", h.Me)
	}
}
