// Route registration for the admin user module.
package user

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires all admin user endpoints on the provided group.
// Every route is behind Auth + RoleRequired(ADMIN).
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	users := group.Group("/users")
	users.Use(middleware.Auth(parser), middleware.RoleRequired("ADMIN"))
	{
		users.GET("", h.List)
		users.GET("/:id", h.Get)
		users.PUT("/:id", h.Update)
		users.DELETE("/:id", h.Deactivate)
		users.PUT("/:id/role", h.AssignRole)
	}
}
