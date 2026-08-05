// Route registration for the activity log module.
package activitylog

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes wires the admin-only activity log route on the group.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	logs := group.Group("/activity-logs")
	logs.Use(middleware.Auth(parser), middleware.RoleRequired("ADMIN"))
	{
		logs.GET("", h.List)
	}
}
