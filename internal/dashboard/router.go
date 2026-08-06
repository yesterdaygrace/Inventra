// Route registration for the dashboard module.
package dashboard

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires all dashboard read endpoints on the provided group.
// They are open to any authenticated caller (no role restriction).
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	dash := group.Group("/dashboard")
	dash.Use(middleware.Auth(parser))
	{
		dash.GET("/summary", h.Summary)
		dash.GET("/activity", h.Activity)
		dash.GET("/inventory-movement", h.InventoryMovement)
		dash.GET("/category-distribution", h.CategoryDistribution)
		dash.GET("/top-selling", h.TopSelling)
	}
}
