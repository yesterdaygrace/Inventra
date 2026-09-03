// Route registration for the inventory module.
package inventory

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires all inventory endpoints on the provided group.
// Read routes (list/ledger/export) are open to any authenticated caller;
// stock movements require their §41 permission codes and are idempotent
// when the caller sends an Idempotency-Key header.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser, idem *middleware.IdempotencyStore) {
	inv := group.Group("/inventory")

	inv.GET("", h.List)
	inv.GET("/ledger", h.Ledger)
	inv.GET("/export", h.Export)
	inv.GET("/reservations", h.ListReservations)

	op := inv.Group("")
	op.Use(middleware.Auth(parser))
	if idem != nil {
		op.Use(idem.Middleware())
	}
	{
		op.POST("/receive", middleware.Permission("inventory.receive"), h.Receive)
		op.POST("/issue", middleware.Permission("inventory.issue"), h.Issue)
		op.POST("/transfers", middleware.Permission("inventory.transfer"), h.Transfer)
		op.POST("/reservations", middleware.Permission("inventory.issue"), h.CreateReservation)
		op.POST("/reservations/:id/release", middleware.Permission("inventory.issue"), h.ReleaseReservation)
		op.POST("/reservations/:id/consume", middleware.Permission("inventory.issue"), h.ConsumeReservation)
	}
}
