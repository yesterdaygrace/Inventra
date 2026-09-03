// Route registration for the category module.
package category

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires all category endpoints on the provided group.
// Write routes are admin-only via RBAC; read routes are available to any
// authenticated caller.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	cats := group.Group("/categories")

	cats.GET("", h.List)
	cats.GET("/export", h.Export)
	cats.GET("/:id", h.Get)

	admin := cats.Group("")
	admin.Use(middleware.Auth(parser))
	{
		admin.POST("", middleware.Permission("category.create"), h.Create)
		admin.PUT("/:id", middleware.Permission("category.update"), h.Update)
		admin.DELETE("/:id", middleware.Permission("category.delete"), h.Delete)
	}
}
