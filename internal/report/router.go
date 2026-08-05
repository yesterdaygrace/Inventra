// Route registration for the report module. All report routes require an
// authenticated user but are not role-restricted (read-model views).
package report

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/middleware"
)

// RegisterRoutes mounts the report read-model endpoints.
func RegisterRoutes(group *gin.RouterGroup, h *Handler, parser middleware.ClaimsParser) {
	rep := group.Group("/reports")
	rep.Use(middleware.Auth(parser))
	{
		rep.GET("/stock-summary", h.Summary)
		rep.GET("/export", h.Export)
		rep.GET("/export-low-stock", h.LowStock)
	}
}
