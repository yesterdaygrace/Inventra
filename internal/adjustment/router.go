// Route registration for the adjustment module.
package adjustment

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires the adjustment endpoints. Submission requires
// inventory.adjust; review (approve/reject) is restricted to ADMIN and
// WAREHOUSE_MANAGER roles per PRD §23.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	adj := group.Group("/adjustments")
	adj.Use(middleware.Auth(parser))
	{
		adj.POST("", middleware.Permission("inventory.adjust"), h.Submit)
		adj.GET("", middleware.Permission("inventory.adjust"), h.List)
		review := adj.Group("")
		review.Use(middleware.RoleRequired("ADMIN", "WAREHOUSE_MANAGER"))
		{
			review.POST("/:id/approve", h.Approve)
			review.POST("/:id/reject", h.Reject)
		}
	}
}
