// HTTP handlers for the dashboard read-model. Each handler validates query
// params and writes the shared response envelope.
package dashboard

import (
	"strconv"

	"github.com/gin-gonic/gin"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/response"
)

// Handler exposes dashboard routes.
type Handler struct {
	svc *Service
}

// NewHandler wires the service into the handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Summary handles GET /dashboard/summary — all KPI cards and widgets.
func (h *Handler) Summary(c *gin.Context) {
	sum, err := h.svc.Summary()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sum)
}

// InventoryMovement handles GET /dashboard/inventory-movement?days=.
func (h *Handler) InventoryMovement(c *gin.Context) {
	days, err := movementDays(c.Query("days"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	payload, err := h.svc.InventoryMovement(days)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, payload)
}

// CategoryDistribution handles GET /dashboard/category-distribution.
func (h *Handler) CategoryDistribution(c *gin.Context) {
	payload, err := h.svc.CategoryDistribution()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, payload)
}

// TopSelling handles GET /dashboard/top-selling?limit=.
func (h *Handler) TopSelling(c *gin.Context) {
	limit, err := topLimit(c.Query("limit"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	payload, err := h.svc.TopSelling(limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, payload)
}

// movementDays parses the optional `days` window. An empty query uses the
// default; a non-numeric, zero, or negative value is rejected.
func movementDays(raw string) (int, error) {
	if raw == "" {
		return DefaultMovementDays, nil
	}
	d, err := strconv.Atoi(raw)
	if err != nil || d < 1 {
		return 0, sharederr.ErrValidation
	}
	return d, nil
}

// topLimit parses the optional `limit`. Empty uses the default; non-numeric
// or non-positive values are rejected (the cap lives in the service).
func topLimit(raw string) (int, error) {
	if raw == "" {
		return 5, nil
	}
	l, err := strconv.Atoi(raw)
	if err != nil || l < 1 {
		return 0, sharederr.ErrValidation
	}
	return l, nil
}
