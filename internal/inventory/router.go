// Route registration for the inventory module.
package inventory

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires all inventory endpoints on the provided group.
// Read routes (list/history/export) are open to any authenticated caller;
// stock movements are restricted to STAFF and ADMIN via RBAC.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	inv := group.Group("/inventory")

	inv.GET("", h.List)
	inv.GET("/transactions", h.Transactions)
	inv.GET("/export", h.Export)

	op := inv.Group("")
	op.Use(middleware.Auth(parser), middleware.RoleRequired("STAFF", "ADMIN"))
	{
		op.POST("/stock-in", h.StockIn)
		op.POST("/stock-out", h.StockOut)
		op.POST("/transfers", h.Transfer)
	}
}
