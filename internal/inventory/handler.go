// HTTP handlers for inventory. Read routes (list/history/export) are open to
// any authenticated caller; stock in/out are restricted to STAFF and ADMIN,
// all returning the shared response envelope.
package inventory

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/audit"
	"inventory/internal/shared/export"
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

// record captures an audit event for a mutation, attaching the acting user
// and request IP. Details are nil-safe.
func (h *Handler) record(c *gin.Context, action, entityID string, details gin.H) {
	eid := entityID
	uid := middleware.UserIDFromContext(c)
	ip := c.ClientIP()
	h.audit.Record(audit.Entry{
		UserID:     &uid,
		Action:     action,
		EntityType: "inventory",
		EntityID:   &eid,
		Details:    details,
		IP:         &ip,
	})
}

type listInventoryRequest struct {
	ProductID string `form:"product_id"`
	Search    string `form:"search"`
	LowStock  bool   `form:"low_stock"`
	Page      int    `form:"page"`
	PerPage   int    `form:"per_page"`
}

type stockRequest struct {
	ProductID string   `json:"product_id" validate:"required"`
	Quantity  int      `json:"quantity" validate:"required,min=1"`
	UnitCost  *float64 `json:"unit_cost"`
	Note      *string  `json:"note"`
}

type transactionsRequest struct {
	ProductID string `form:"product_id"`
	Type      string `form:"type"`
	Page      int    `form:"page"`
	PerPage   int    `form:"per_page"`
}

type inventoryEnvelope struct {
	ProductID   uuid.UUID `json:"product_id"`
	ProductSKU  string    `json:"product_sku"`
	ProductName string    `json:"product_name"`
	Quantity    int       `json:"quantity"`
	UpdatedAt   string    `json:"updated_at"`
}

type transactionEnvelope struct {
	ID          uuid.UUID  `json:"id"`
	ProductID   uuid.UUID  `json:"product_id"`
	ProductSKU  string     `json:"product_sku"`
	ProductName string     `json:"product_name"`
	Type        string     `json:"type"`
	Quantity    int        `json:"quantity"`
	UnitCost    *float64   `json:"unit_cost,omitempty"`
	Note        *string    `json:"note,omitempty"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	CreatedAt   string     `json:"created_at"`
}

func parseUUID(raw string) (uuid.UUID, bool) {
	if strings.TrimSpace(raw) == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

// List handles GET /inventory — joined product/stock view with filters.
func (h *Handler) List(c *gin.Context) {
	var req listInventoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	q := ListQuery{
		Search:   req.Search,
		LowStock: req.LowStock,
		Page:     req.Page,
		PerPage:  req.PerPage,
	}
	if req.ProductID != "" {
		id, ok := parseUUID(req.ProductID)
		if !ok {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		q.ProductID = id
	}

	views, total, err := h.svc.List(q)
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]inventoryEnvelope, 0, len(views))
	for _, v := range views {
		items = append(items, inventoryEnvelope{
			ProductID:   v.ProductID,
			ProductSKU:  v.ProductSKU,
			ProductName: v.ProductName,
			Quantity:    v.Quantity,
			UpdatedAt:   v.UpdatedAt,
		})
	}

	response.Paginated(c, items, &response.Pagination{
		Page:       clampPage(req.Page),
		PerPage:    clampPerPage(req.PerPage),
		Total:      total,
		TotalPages: int((total + int64(clampPerPage(req.PerPage)) - 1) / int64(clampPerPage(req.PerPage))),
	})
}

// StockIn handles POST /inventory/stock-in.
func (h *Handler) StockIn(c *gin.Context) {
	h.mutate(c, "IN")
}

// StockOut handles POST /inventory/stock-out.
func (h *Handler) StockOut(c *gin.Context) {
	h.mutate(c, "OUT")
}

func (h *Handler) mutate(c *gin.Context, kind string) {
	var req stockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	pid, ok := parseUUID(req.ProductID)
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	userID := middleware.UserIDFromContext(c)
	uid := userID

	var inv *Inventory
	var err error
	if kind == "IN" {
		inv, err = h.svc.StockIn(Movement{
			ProductID: pid,
			Type:      "IN",
			Quantity:  req.Quantity,
			UnitCost:  req.UnitCost,
			Note:      req.Note,
			UserID:    &uid,
		})
	} else {
		inv, err = h.svc.StockOut(Movement{
			ProductID: pid,
			Type:      "OUT",
			Quantity:  req.Quantity,
			UnitCost:  req.UnitCost,
			Note:      req.Note,
			UserID:    &uid,
		})
	}
	if err != nil {
		response.Error(c, err)
		return
	}

	h.record(c, "STOCK_"+kind, pid.String(), gin.H{
		"product_id": pid.String(),
		"quantity":   req.Quantity,
		"note":       noteOrNil(req.Note),
	})
	response.OK(c, gin.H{
		"product_id": inv.ProductID,
		"quantity":   inv.Quantity,
		"updated_at": inv.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

func noteOrNil(n *string) any {
	if n == nil {
		return nil
	}
	return *n
}

// Transactions handles GET /inventory/transactions — paginated history.
func (h *Handler) Transactions(c *gin.Context) {
	var req transactionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	q := TransactionQuery{
		Type:    req.Type,
		Page:    req.Page,
		PerPage: req.PerPage,
	}
	if req.ProductID != "" {
		id, ok := parseUUID(req.ProductID)
		if !ok {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		q.ProductID = id
	}

	views, total, err := h.svc.Transactions(q)
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]transactionEnvelope, 0, len(views))
	for _, v := range views {
		items = append(items, transactionEnvelope{
			ID:          v.ID,
			ProductID:   v.ProductID,
			ProductSKU:  v.ProductSKU,
			ProductName: v.ProductName,
			Type:        v.Type,
			Quantity:    v.Quantity,
			UnitCost:    v.UnitCost,
			Note:        v.Note,
			UserID:      v.UserID,
			CreatedAt:   v.CreatedAt,
		})
	}

	response.Paginated(c, items, &response.Pagination{
		Page:       clampPage(req.Page),
		PerPage:    clampPerPage(req.PerPage),
		Total:      total,
		TotalPages: int((total + int64(clampPerPage(req.PerPage)) - 1) / int64(clampPerPage(req.PerPage))),
	})
}

// Export handles GET /inventory/export — CSV of current stock levels.
func (h *Handler) Export(c *gin.Context) {
	views, _, err := h.svc.List(ListQuery{PerPage: 1000})
	if err != nil {
		response.Error(c, err)
		return
	}

	rows := make([][]string, 0, len(views))
	for _, v := range views {
		rows = append(rows, []string{
			v.ProductID.String(),
			v.ProductSKU,
			v.ProductName,
			strconv.Itoa(v.Quantity),
			v.UpdatedAt,
		})
	}

	export.SetAttachment(c, "inventory")
	_ = export.WriteCSV(c.Writer, []string{"product_id", "sku", "name", "quantity", "updated_at"}, rows)
}

func clampPage(p int) int {
	if p < 1 {
		return 1
	}
	return p
}

func clampPerPage(p int) int {
	if p < 1 || p > 100 {
		return 20
	}
	return p
}
