// HTTP handlers for inventory. Read routes (list/history/export) are open to
// any authenticated caller; stock in/out are restricted to STAFF and ADMIN,
// all returning the shared response envelope.
package inventory

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"inventory/internal/shared/audit"
	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/export"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/response"
	"inventory/internal/shared/validator"
)

type Handler struct {
	svc   *Service
	val   *validator.Validator
	audit audit.Recorder
	zlog  *zap.Logger
}

func NewHandler(svc *Service, val *validator.Validator) *Handler {
	return &Handler{svc: svc, val: val, audit: audit.Nop{}, zlog: zap.NewNop()}
}

// SetAudit wires an audit recorder (nil-safe; Nop by default).
func (h *Handler) SetAudit(r audit.Recorder) {
	if r != nil {
		h.audit = r
	}
}

// SetLogger wires a structured logger (nil-safe; Nop by default).
func (h *Handler) SetLogger(l *zap.Logger) {
	if l != nil {
		h.zlog = l
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
	ProductID   string `form:"product_id"`
	Search      string `form:"search"`
	LowStock    bool   `form:"low_stock"`
	WarehouseID string `form:"warehouse_id"`
	Page        int    `form:"page"`
	PerPage     int    `form:"per_page"`
}

type stockRequest struct {
	ProductID   string   `json:"product_id" validate:"required"`
	Quantity    int      `json:"quantity" validate:"required,min=1"`
	UnitCost    *float64 `json:"unit_cost"`
	Note        *string  `json:"note"`
	WarehouseID *string  `json:"warehouse_id"`
}

type transferRequest struct {
	ProductID       string  `json:"product_id" validate:"required"`
	Quantity        int     `json:"quantity" validate:"required,min=1"`
	FromWarehouseID string  `json:"from_warehouse_id" validate:"required"`
	ToWarehouseID   string  `json:"to_warehouse_id" validate:"required"`
	Note            *string `json:"note"`
}

type transactionsRequest struct {
	ProductID   string `form:"product_id"`
	Type        string `form:"type"`
	WarehouseID string `form:"warehouse_id"`
	Page        int    `form:"page"`
	PerPage     int    `form:"per_page"`
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
	WarehouseID *uuid.UUID `json:"warehouse_id,omitempty"`
	TransferID  *uuid.UUID `json:"transfer_id,omitempty"`
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
// @Tags inventory
// @Summary List current stock levels
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param product_id query string false "Filter by product UUID"
// @Param search query string false "Search by product name/SKU"
// @Param low_stock query boolean false "Only low-stock items"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /inventory [get]
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
	if req.WarehouseID != "" {
		id, ok := parseUUID(req.WarehouseID)
		if !ok {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		q.WarehouseID = &id
	}

	views, total, err := h.svc.List(c.Request.Context(), q)
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
// @Tags inventory
// @Summary Record stock-in movement
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body stockRequest true "Stock-in payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Router /inventory/stock-in [post]
func (h *Handler) StockIn(c *gin.Context) {
	h.mutate(c, "IN")
}

// StockOut handles POST /inventory/stock-out.
// @Tags inventory
// @Summary Record stock-out movement
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body stockRequest true "Stock-out payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /inventory/stock-out [post]
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

	var whID *uuid.UUID
	if req.WarehouseID != nil {
		id, ok := parseUUID(*req.WarehouseID)
		if !ok {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		whID = &id
	}

	userID := middleware.UserIDFromContext(c)
	uid := userID

	var inv *Inventory
	var err error
	if kind == "IN" {
		inv, err = h.svc.StockIn(c.Request.Context(), Movement{
			ProductID:   pid,
			Type:        "IN",
			Quantity:    req.Quantity,
			UnitCost:    req.UnitCost,
			Note:        req.Note,
			UserID:      &uid,
			WarehouseID: whID,
		})
	} else {
		inv, err = h.svc.StockOut(c.Request.Context(), Movement{
			ProductID:   pid,
			Type:        "OUT",
			Quantity:    req.Quantity,
			UnitCost:    req.UnitCost,
			Note:        req.Note,
			UserID:      &uid,
			WarehouseID: whID,
		})
	}
	if err != nil {
		response.Error(c, err)
		return
	}

	details := gin.H{
		"product_id": pid.String(),
		"quantity":   req.Quantity,
		"note":       noteOrNil(req.Note),
	}
	if whID != nil {
		details["warehouse_id"] = whID.String()
	}
	h.record(c, "STOCK_"+kind, pid.String(), details)
	response.OK(c, gin.H{
		"product_id": inv.ProductID,
		"quantity":   inv.Quantity,
		"updated_at": inv.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

// Transfer handles POST /inventory/transfers — moves stock between warehouses.
// @Tags inventory
// @Summary Transfer stock between warehouses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body transferRequest true "Transfer payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /inventory/transfers [post]
func (h *Handler) Transfer(c *gin.Context) {
	var req transferRequest
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
	from, ok := parseUUID(req.FromWarehouseID)
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	to, ok := parseUUID(req.ToWarehouseID)
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	userID := middleware.UserIDFromContext(c)
	uid := userID

	inv, err := h.svc.Transfer(c.Request.Context(), Transfer{
		ProductID:       pid,
		FromWarehouseID: from,
		ToWarehouseID:   to,
		Quantity:        req.Quantity,
		Note:            req.Note,
		UserID:          &uid,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	h.record(c, "TRANSFER", pid.String(), gin.H{
		"product_id":        pid.String(),
		"from_warehouse_id": from.String(),
		"to_warehouse_id":   to.String(),
		"quantity":          req.Quantity,
		"note":              noteOrNil(req.Note),
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
// @Tags inventory
// @Summary List inventory movement history
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param product_id query string false "Filter by product UUID"
// @Param type query string false "Movement type" Enums(IN, OUT)
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /inventory/transactions [get]
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
	if req.WarehouseID != "" {
		id, ok := parseUUID(req.WarehouseID)
		if !ok {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		q.WarehouseID = &id
	}

	views, total, err := h.svc.Transactions(c.Request.Context(), q)
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
			WarehouseID: v.WarehouseID,
			TransferID:  v.TransferID,
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
// @Tags inventory
// @Summary Export current stock levels as CSV
// @Accept json
// @Produce text/csv
// @Security BearerAuth
// @Success 200
// @Failure 401 {object} response.Body
// @Router /inventory/export [get]
func (h *Handler) Export(c *gin.Context) {
	views, _, err := h.svc.List(c.Request.Context(), ListQuery{PerPage: 1000})
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
	headers := []string{"product_id", "sku", "name", "quantity", "updated_at"}
	if err := export.WriteCSV(c.Writer, headers, rows); err != nil {
		h.zlog.Warn("csv export failed", zap.Error(err))
	}
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
