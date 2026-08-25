// Route registration for the cycle counting module.
package cyclecount

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires the cycle count endpoints. Counting is staff work
// (PRD §4), gated by inventory.adjust; variance corrections flow through
// the adjustment approval workflow before stock moves.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	cc := group.Group("/cycle-counts")
	cc.Use(middleware.Auth(parser), middleware.Permission("inventory.adjust"))
	{
		cc.POST("", h.CreatePlan)
		cc.GET("", h.ListPlans)
		cc.GET("/:id", h.GetPlan)
		cc.POST("/:id/items/:itemId/count", h.RecordCount)
	}
}
