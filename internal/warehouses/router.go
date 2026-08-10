// Route registration for the warehouse module.
package warehouses

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires all warehouse endpoints on the provided group.
// Write routes are admin-only via RBAC; read routes are available to any
// authenticated caller.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	wh := group.Group("/warehouses")

	wh.GET("", h.List)
	wh.GET("/:id", h.Get)

	admin := wh.Group("")
	admin.Use(middleware.Auth(parser), middleware.RoleRequired("ADMIN"))
	{
		admin.POST("", h.Create)
		admin.PUT("/:id", h.Update)
		admin.DELETE("/:id", h.Delete)
	}
}
