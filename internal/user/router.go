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
	users.Use(middleware.Auth(parser))
	{
		users.GET("", middleware.Permission("user.manage"), h.List)
		users.GET("/:id", middleware.Permission("user.manage"), h.Get)
		users.PUT("/:id", middleware.Permission("user.manage"), h.Update)
		users.DELETE("/:id", middleware.Permission("user.manage"), h.Deactivate)
		users.PUT("/:id/role", middleware.Permission("user.manage"), h.AssignRole)
	}
}
