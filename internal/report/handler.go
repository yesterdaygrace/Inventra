// HTTP handlers for the report read-model. Summary returns the JSON envelope;
// export streams the same numbers as a CSV attachment via the shared export util.
package report

import (
	"github.com/gin-gonic/gin"

	"inventory/internal/shared/export"
	"inventory/internal/shared/response"
)

// Handler exposes report routes.
type Handler struct {
	svc *Service
}

// NewHandler wires the service into the handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Summary handles GET /reports/stock-summary.
// @Tags reports
// @Summary Stock summary report
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /reports/stock-summary [get]
func (h *Handler) Summary(c *gin.Context) {
	summary, err := h.svc.Summary()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, summary)
}

// Export handles GET /reports/export — streams category totals as CSV.
// @Tags reports
// @Summary Export category stock totals as CSV
// @Accept json
// @Produce text/csv
// @Security BearerAuth
// @Success 200
// @Failure 401 {object} response.Body
// @Router /reports/export [get]
func (h *Handler) Export(c *gin.Context) {
	headers, rows, err := h.svc.ExportRows()
	if err != nil {
		response.Error(c, err)
		return
	}
	export.SetAttachment(c, "reports")
	if err := export.WriteCSV(c.Writer, headers, rows); err != nil {
		return
	}
}

// LowStock handles GET /reports/export-low-stock — streams low-stock items as CSV.
// @Tags reports
// @Summary Export low-stock items as CSV
// @Accept json
// @Produce text/csv
// @Security BearerAuth
// @Success 200
// @Failure 401 {object} response.Body
// @Router /reports/export-low-stock [get]
func (h *Handler) LowStock(c *gin.Context) {
	headers, rows, err := h.svc.LowStockRows()
	if err != nil {
		response.Error(c, err)
		return
	}
	export.SetAttachment(c, "reports-low-stock")
	if err := export.WriteCSV(c.Writer, headers, rows); err != nil {
		return
	}
}
