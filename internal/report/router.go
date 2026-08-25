// Route registration for the report module. Summary views require
// report.read; CSV exports require report.export.
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
		rep.GET("/stock-summary", middleware.Permission("report.read"), h.Summary)
		rep.GET("/export", middleware.Permission("report.export"), h.Export)
		rep.GET("/export-low-stock", middleware.Permission("report.export"), h.LowStock)
	}
}
