// HTTP handlers for cycle counting.
package cyclecount

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"inventory/internal/shared/audit"
	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/response"
	"inventory/internal/shared/validator"
)

type Handler struct {
	svc   *Service
	val   *validator.Validator
	audit audit.Recorder
}

func NewHandler(svc *Service, val *validator.Validator) *Handler {
	return &Handler{svc: svc, val: val, audit: audit.Nop{}}
}

// SetAudit wires an audit recorder (nil-safe; Nop by default).
func (h *Handler) SetAudit(r audit.Recorder) {
	if r != nil {
		h.audit = r
	}
}

type createPlanRequest struct {
	WarehouseID string   `json:"warehouse_id" validate:"required"`
	Name        string   `json:"name" validate:"required"`
	ProductIDs  []string `json:"product_ids" validate:"required,min=1"`
}

type recordCountRequest struct {
	CountedQuantity int `json:"counted_quantity" validate:"min=0"`
}

func parseUUID(raw string) (uuid.UUID, bool) {
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

// CreatePlan handles POST /cycle-counts.
func (h *Handler) CreatePlan(c *gin.Context) {
	var req createPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	wid, ok := parseUUID(req.WarehouseID)
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	pids := make([]uuid.UUID, 0, len(req.ProductIDs))
	for _, raw := range req.ProductIDs {
		pid, ok := parseUUID(raw)
		if !ok {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		pids = append(pids, pid)
	}
	userID := middleware.UserIDFromContext(c)

	plan, err := h.svc.CreatePlan(c.Request.Context(), CreatePlanInput{
		WarehouseID: wid,
		Name:        req.Name,
		ProductIDs:  pids,
		CreatedBy:   userID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	h.audit.Record(audit.EntryFromContext(c, audit.Entry{
		Action:     "CYCLE_COUNT_CREATED",
		EntityType: "cycle_count",
		EntityID:   &[]string{plan.ID.String()}[0],
		Details:    gin.H{"name": plan.Name, "items": len(pids)},
	}))
	response.Created(c, gin.H{"id": plan.ID, "status": plan.Status})
}

// ListPlans handles GET /cycle-counts.
func (h *Handler) ListPlans(c *gin.Context) {
	views, total, err := h.svc.ListPlans(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	items := make([]gin.H, 0, len(views))
	for _, v := range views {
		items = append(items, gin.H{
			"id":             v.ID,
			"warehouse_id":   v.WarehouseID,
			"name":           v.Name,
			"status":         v.Status,
			"total_items":    v.TotalItems,
			"counted_items":  v.CountedItems,
			"variance_items": v.VarianceItems,
			"created_at":     v.CreatedAt,
		})
	}
	response.Paginated(c, items, &response.Pagination{Page: 1, PerPage: 100, Total: total, TotalPages: 1})
}

// GetPlan handles GET /cycle-counts/:id — plan header plus items.
func (h *Handler) GetPlan(c *gin.Context) {
	id, ok := parseUUID(c.Param("id"))
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	plan, err := h.svc.repo.GetPlan(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.svc.PlanItems(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	itemPayload := make([]gin.H, 0, len(items))
	for _, it := range items {
		itemPayload = append(itemPayload, gin.H{
			"id":               it.ID,
			"product_id":       it.ProductID,
			"product_sku":      it.ProductSKU,
			"product_name":     it.ProductName,
			"system_quantity":  it.SystemQuantity,
			"counted_quantity": it.CountedQuantity,
			"status":           it.Status,
			"adjustment_id":    it.AdjustmentID,
			"counted_at":       it.CountedAt,
		})
	}
	response.OK(c, gin.H{
		"id":           plan.ID,
		"warehouse_id": plan.WarehouseID,
		"name":         plan.Name,
		"status":       plan.Status,
		"items":        itemPayload,
	})
}

// RecordCount handles POST /cycle-counts/:id/items/:itemId/count.
func (h *Handler) RecordCount(c *gin.Context) {
	itemID, ok := parseUUID(c.Param("itemId"))
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	var req recordCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	userID := middleware.UserIDFromContext(c)

	it, err := h.svc.RecordCount(c.Request.Context(), RecordCountInput{
		ItemID:          itemID,
		CountedQuantity: req.CountedQuantity,
		CountedBy:       userID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	details := gin.H{"item_id": it.ID.String(), "counted_quantity": *it.CountedQuantity}
	if it.AdjustmentID != nil {
		details["adjustment_id"] = it.AdjustmentID.String()
	}
	h.audit.Record(audit.EntryFromContext(c, audit.Entry{
		Action:     "CYCLE_COUNT_RECORDED",
		EntityType: "cycle_count_item",
		EntityID:   &[]string{it.ID.String()}[0],
		Details:    details,
	}))

	resp := gin.H{"id": it.ID, "counted_quantity": *it.CountedQuantity}
	if it.AdjustmentID != nil {
		resp["adjustment_id"] = it.AdjustmentID
	}
	response.OK(c, resp)
}
