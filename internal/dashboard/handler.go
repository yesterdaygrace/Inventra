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
// @Tags dashboard
// @Summary Dashboard KPIs and widgets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /dashboard/summary [get]
func (h *Handler) Summary(c *gin.Context) {
	sum, err := h.svc.Summary(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sum)
}

// activityQuery binds the pagination params for the activity feed.
type activityQuery struct {
	Page    int `form:"page"`
	PerPage int `form:"per_page"`
}

// Activity handles GET /dashboard/activity — paginated recent audit events.
// @Tags dashboard
// @Summary Recent activity feed
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param per_page query int false "Items per page"
// @Success 200 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /dashboard/activity [get]
func (h *Handler) Activity(c *gin.Context) {
	var q activityQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	items, total, err := h.svc.Activities(c.Request.Context(), q.Page, q.PerPage)
	if err != nil {
		response.Error(c, err)
		return
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	perPage := q.PerPage
	if perPage < 1 {
		perPage = 20
	}
	totalPages := 0
	if perPage > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}
	response.Paginated(c, items, &response.Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// InventoryMovement handles GET /dashboard/inventory-movement?days=.
// @Tags dashboard
// @Summary Inventory movement over time
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param days query int false "Lookback window in days" default(30)
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /dashboard/inventory-movement [get]
func (h *Handler) InventoryMovement(c *gin.Context) {
	days, err := movementDays(c.Query("days"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	payload, err := h.svc.InventoryMovement(c.Request.Context(), days)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, payload)
}

// CategoryDistribution handles GET /dashboard/category-distribution.
// @Tags dashboard
// @Summary Category distribution
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /dashboard/category-distribution [get]
func (h *Handler) CategoryDistribution(c *gin.Context) {
	payload, err := h.svc.CategoryDistribution(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, payload)
}

// TopSelling handles GET /dashboard/top-selling?limit=.
// @Tags dashboard
// @Summary Top selling products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Number of results" default(5)
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /dashboard/top-selling [get]
func (h *Handler) TopSelling(c *gin.Context) {
	limit, err := topLimit(c.Query("limit"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	payload, err := h.svc.TopSelling(c.Request.Context(), limit)
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
