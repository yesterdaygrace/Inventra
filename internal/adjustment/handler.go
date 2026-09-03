// HTTP handlers for the adjustment workflow.
package adjustment

import (
	"strings"

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

type submitRequest struct {
	ProductID       string  `json:"product_id" validate:"required"`
	WarehouseID     string  `json:"warehouse_id" validate:"required"`
	CountedQuantity int     `json:"counted_quantity" validate:"min=0"`
	Reason          string  `json:"reason" validate:"required"`
	Note            *string `json:"note"`
}

func parseUUID(raw string) (uuid.UUID, bool) {
	if strings.TrimSpace(raw) == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

// Submit handles POST /adjustments.
func (h *Handler) Submit(c *gin.Context) {
	var req submitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	pid, ok1 := parseUUID(req.ProductID)
	wid, ok2 := parseUUID(req.WarehouseID)
	if !ok1 || !ok2 {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	userID := middleware.UserIDFromContext(c)

	// The performer's permission set decides auto-approval eligibility;
	// the value threshold still gates it inside the service.
	perms := middleware.PermissionsFromContext(c)
	autoApprove := false
	for _, p := range perms {
		if p == "inventory.adjust" {
			autoApprove = true
			break
		}
	}

	a, err := h.svc.Submit(c.Request.Context(), SubmitInput{
		ProductID:       pid,
		WarehouseID:     wid,
		CountedQuantity: req.CountedQuantity,
		Reason:          req.Reason,
		Note:            req.Note,
		RequestedBy:     userID,
		AutoApprove:     autoApprove,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	h.audit.Record(audit.EntryFromContext(c, audit.Entry{
		Action:     "ADJUSTMENT_SUBMITTED",
		EntityType: "adjustment",
		EntityID:   &[]string{a.ID.String()}[0],
		Details: gin.H{
			"product_id":       pid.String(),
			"warehouse_id":     wid.String(),
			"system_quantity":  a.SystemQuantity,
			"counted_quantity": a.CountedQuantity,
			"reason":           a.Reason,
			"status":           a.Status,
		},
	}))
	response.Created(c, gin.H{
		"id":               a.ID,
		"status":           a.Status,
		"system_quantity":  a.SystemQuantity,
		"counted_quantity": a.CountedQuantity,
	})
}

// List handles GET /adjustments?status=PENDING — the review queue.
func (h *Handler) List(c *gin.Context) {
	q := ListQuery{Status: c.Query("status"), Page: pageOf(c.Query("page")), PerPage: pageOf(c.Query("per_page"))}
	views, total, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		response.Error(c, err)
		return
	}
	items := make([]gin.H, 0, len(views))
	for _, v := range views {
		items = append(items, gin.H{
			"id":               v.ID,
			"product_id":       v.ProductID,
			"product_sku":      v.ProductSKU,
			"product_name":     v.ProductName,
			"warehouse_id":     v.WarehouseID,
			"system_quantity":  v.SystemQuantity,
			"counted_quantity": v.CountedQuantity,
			"reason":           v.Reason,
			"note":             v.Note,
			"status":           v.Status,
			"applied_value":    v.AppliedValue,
			"created_at":       v.CreatedAt,
		})
	}
	per := clampPer(q.PerPage)
	response.Paginated(c, items, &response.Pagination{
		Page: clampP(q.Page), PerPage: per, Total: total,
		TotalPages: int((total + int64(per) - 1) / int64(per)),
	})
}

// Approve handles POST /adjustments/:id/approve — applies the correction.
func (h *Handler) Approve(c *gin.Context) {
	h.review(c, true)
}

// Reject handles POST /adjustments/:id/reject.
func (h *Handler) Reject(c *gin.Context) {
	h.review(c, false)
}

func (h *Handler) review(c *gin.Context, approve bool) {
	id, ok := parseUUID(c.Param("id"))
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	reviewer := middleware.UserIDFromContext(c)

	var a *Adjustment
	var err error
	if approve {
		a, err = h.svc.Approve(c.Request.Context(), id, reviewer)
	} else {
		a, err = h.svc.Reject(c.Request.Context(), id, reviewer)
	}
	if err != nil {
		response.Error(c, err)
		return
	}

	action := "ADJUSTMENT_REJECTED"
	if approve {
		action = "ADJUSTMENT_APPROVED"
	}
	h.audit.Record(audit.EntryFromContext(c, audit.Entry{
		Action:     action,
		EntityType: "adjustment",
		EntityID:   &[]string{a.ID.String()}[0],
		Details:    gin.H{"status": a.Status, "counted_quantity": a.CountedQuantity},
	}))
	response.OK(c, gin.H{"id": a.ID, "status": a.Status})
}

func pageOf(raw string) int {
	n := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

func clampP(p int) int {
	if p < 1 {
		return 1
	}
	return p
}

func clampPer(p int) int {
	if p < 1 || p > 100 {
		return 20
	}
	return p
}
