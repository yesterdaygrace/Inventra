// Route registration for the product module.
package product

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires all product endpoints on the provided group.
// Write routes are admin-only via RBAC; read routes (list/get/export) are
// available to any authenticated caller.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	prods := group.Group("/products")

	prods.GET("", h.List)
	prods.GET("/export", h.Export)
	prods.GET("/:id", h.Get)

	admin := prods.Group("")
	admin.Use(middleware.Auth(parser))
	{
		admin.POST("", middleware.Permission("product.create"), h.Create)
		admin.PUT("/:id", middleware.Permission("product.update"), h.Update)
		admin.DELETE("/:id", middleware.Permission("product.delete"), h.Delete)
	}
}
