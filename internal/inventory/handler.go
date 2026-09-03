// HTTP handlers for inventory. Read routes (list/history/export) are open to
// any authenticated caller; stock in/out are restricted to STAFF and ADMIN,
// all returning the shared response envelope.
package inventory

import (
	"time"
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

type listInventoryRequest struct {
	ProductID   string `form:"product_id"`
	Search      string `form:"search"`
	LowStock    bool   `form:"low_stock"`
	WarehouseID string `form:"warehouse_id"`
	Page        int    `form:"page"`
	PerPage     int    `form:"per_page"`
}

type stockRequest struct {
	ProductID     string   `json:"product_id" validate:"required"`
	Quantity      int      `json:"quantity" validate:"required,min=1"`
	UnitCost      *float64 `json:"unit_cost"`
	Note          *string  `json:"note"`
	WarehouseID   *string  `json:"warehouse_id"`
	ReferenceType *string  `json:"reference_type"`
	ReferenceID   *string  `json:"reference_id"`
	Reason        *string  `json:"reason"`
}

type transferRequest struct {
	ProductID       string  `json:"product_id" validate:"required"`
	Quantity        int     `json:"quantity" validate:"required,min=1"`
	FromWarehouseID string  `json:"from_warehouse_id" validate:"required"`
	ToWarehouseID   string  `json:"to_warehouse_id" validate:"required"`
	Note            *string `json:"note"`
	ReferenceType   *string `json:"reference_type"`
	ReferenceID     *string `json:"reference_id"`
	Reason          *string `json:"reason"`
}

type transactionsRequest struct {
	ProductID   string `form:"product_id"`
	Type        string `form:"type"`
	WarehouseID string `form:"warehouse_id"`
	Page        int    `form:"page"`
	PerPage     int    `form:"per_page"`
}

type inventoryEnvelope struct {
	ProductID        uuid.UUID `json:"product_id"`
	ProductSKU       string    `json:"product_sku"`
	ProductName      string    `json:"product_name"`
	Quantity         int       `json:"quantity"`
	ReservedQuantity int       `json:"reserved_quantity"`
	Version          int       `json:"version"`
	UpdatedAt        string    `json:"updated_at"`
}

type ledgerEnvelope struct {
	ID              uuid.UUID  `json:"id"`
	ProductID       uuid.UUID  `json:"product_id"`
	ProductSKU      string     `json:"product_sku"`
	ProductName     string     `json:"product_name"`
	TransactionType string     `json:"transaction_type"`
	Direction       string     `json:"direction"`
	Quantity        int        `json:"quantity"`
	Balance         int        `json:"balance"`
	UnitCost        *float64   `json:"unit_cost,omitempty"`
	TotalCost       *float64   `json:"total_cost,omitempty"`
	Note            *string    `json:"note,omitempty"`
	UserID          *uuid.UUID `json:"performed_by,omitempty"`
	WarehouseID     uuid.UUID  `json:"warehouse_id"`
	TransferID      *uuid.UUID `json:"transfer_id,omitempty"`
	ReferenceType   *string    `json:"reference_type,omitempty"`
	ReferenceID     *string    `json:"reference_id,omitempty"`
	Reason          *string    `json:"reason,omitempty"`
	CreatedAt       string     `json:"created_at"`
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
			ProductID:        v.ProductID,
			ProductSKU:       v.ProductSKU,
			ProductName:      v.ProductName,
			Quantity:         v.Quantity,
			ReservedQuantity: v.ReservedQuantity,
			Version:          v.Version,
			UpdatedAt:        v.UpdatedAt,
		})
	}

	response.Paginated(c, items, &response.Pagination{
		Page:       clampPage(req.Page),
		PerPage:    clampPerPage(req.PerPage),
		Total:      total,
		TotalPages: int((total + int64(clampPerPage(req.PerPage)) - 1) / int64(clampPerPage(req.PerPage))),
	})
}

// Receive handles POST /inventory/receive.
// @Tags inventory
// @Summary Receive stock into a warehouse
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body stockRequest true "Receive payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Router /inventory/receive [post]
func (h *Handler) Receive(c *gin.Context) {
	h.mutate(c, LedgerReceive)
}

// Issue handles POST /inventory/issue.
// @Tags inventory
// @Summary Issue stock out of a warehouse
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body stockRequest true "Issue payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /inventory/issue [post]
func (h *Handler) Issue(c *gin.Context) {
	h.mutate(c, LedgerIssue)
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
	if kind == LedgerReceive {
		inv, err = h.svc.Receive(c.Request.Context(), Movement{
			ProductID:     pid,
			Type:          LedgerReceive,
			Quantity:      req.Quantity,
			UnitCost:      req.UnitCost,
			Note:          req.Note,
			UserID:        &uid,
			WarehouseID:   whID,
			ReferenceType: req.ReferenceType,
			ReferenceID:   req.ReferenceID,
			Reason:        req.Reason,
		})
	} else {
		inv, err = h.svc.Issue(c.Request.Context(), Movement{
			ProductID:     pid,
			Type:          LedgerIssue,
			Quantity:      req.Quantity,
			UnitCost:      req.UnitCost,
			Note:          req.Note,
			UserID:        &uid,
			WarehouseID:   whID,
			ReferenceType: req.ReferenceType,
			ReferenceID:   req.ReferenceID,
			Reason:        req.Reason,
		})
	}
	if err != nil {
		response.Error(c, err)
		return
	}

	details := gin.H{
		"product_id":       pid.String(),
		"quantity":         req.Quantity,
		"transaction_type": kind,
		"note":             noteOrNil(req.Note),
	}
	if whID != nil {
		details["warehouse_id"] = whID.String()
	}
	if req.ReferenceType != nil {
		details["reference_type"] = *req.ReferenceType
	}
	if req.ReferenceID != nil {
		details["reference_id"] = *req.ReferenceID
	}

	before := inv.Quantity - req.Quantity
	if kind == LedgerIssue {
		before = inv.Quantity + req.Quantity
	}
	h.audit.Record(audit.EntryFromContext(c, audit.Entry{
		Action:     "INVENTORY_" + kind,
		EntityType: "inventory",
		EntityID:   &[]string{pid.String()}[0],
		Details:    details,
		Reason:     req.Reason,
		BeforeData: gin.H{"quantity": before},
		AfterData:  gin.H{"quantity": inv.Quantity},
	}))
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
		ReferenceType:   req.ReferenceType,
		ReferenceID:     req.ReferenceID,
		Reason:          req.Reason,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	details := gin.H{
		"product_id":        pid.String(),
		"from_warehouse_id": from.String(),
		"to_warehouse_id":   to.String(),
		"quantity":          req.Quantity,
		"note":              noteOrNil(req.Note),
	}
	before := inv.Quantity - req.Quantity
	h.audit.Record(audit.EntryFromContext(c, audit.Entry{
		Action:     "TRANSFER",
		EntityType: "inventory",
		EntityID:   &[]string{pid.String()}[0],
		Details:    details,
		Reason:     req.Reason,
		BeforeData: gin.H{"destination_quantity": before},
		AfterData:  gin.H{"destination_quantity": inv.Quantity},
	}))
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

// Ledger handles GET /inventory/ledger — the append-only movement history
// with running balances (PRD §53).
// @Tags inventory
// @Summary List the inventory ledger
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param product_id query string false "Filter by product UUID"
// @Param warehouse_id query string false "Filter by warehouse UUID"
// @Param type query string false "Transaction type" Enums(OPENING_BALANCE,RECEIVE,ISSUE,TRANSFER_IN,TRANSFER_OUT,ADJUSTMENT,RETURN)
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /inventory/ledger [get]
func (h *Handler) Ledger(c *gin.Context) {
	var req transactionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	q := LedgerQuery{
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

	views, total, err := h.svc.Ledger(c.Request.Context(), q)
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]ledgerEnvelope, 0, len(views))
	for _, v := range views {
		items = append(items, ledgerEnvelope{
			ID:              v.ID,
			ProductID:       v.ProductID,
			ProductSKU:      v.ProductSKU,
			ProductName:     v.ProductName,
			TransactionType: v.TransactionType,
			Direction:       v.Direction,
			Quantity:        v.Quantity,
			Balance:         v.Balance,
			UnitCost:        v.UnitCost,
			TotalCost:       v.TotalCost,
			Note:            v.Note,
			UserID:          v.UserID,
			WarehouseID:     v.WarehouseID,
			TransferID:      v.TransferID,
			ReferenceType:   v.ReferenceType,
			ReferenceID:     v.ReferenceID,
			Reason:          v.Reason,
			CreatedAt:       v.CreatedAt,
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

type reservationRequest struct {
	ProductID     string  `json:"product_id" validate:"required"`
	WarehouseID   string  `json:"warehouse_id" validate:"required"`
	Quantity      int     `json:"quantity" validate:"required,min=1"`
	ReferenceType string  `json:"reference_type" validate:"required"`
	ReferenceID   string  `json:"reference_id" validate:"required"`
	ExpiresAt     *string `json:"expires_at"`
}

type reservationsRequest struct {
	ProductID   string `form:"product_id"`
	WarehouseID string `form:"warehouse_id"`
	Status      string `form:"status"`
	Page        int    `form:"page"`
	PerPage     int    `form:"per_page"`
}

type reservationEnvelope struct {
	ID            uuid.UUID `json:"id"`
	ProductID     uuid.UUID `json:"product_id"`
	ProductSKU    string    `json:"product_sku"`
	ProductName   string    `json:"product_name"`
	WarehouseID   uuid.UUID `json:"warehouse_id"`
	Quantity      int       `json:"quantity"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   string    `json:"reference_id"`
	Status        string    `json:"status"`
	ExpiresAt     *string   `json:"expires_at,omitempty"`
	CreatedAt     string    `json:"created_at"`
}

// CreateReservation handles POST /inventory/reservations.
func (h *Handler) CreateReservation(c *gin.Context) {
	var req reservationRequest
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

	var expires *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		expires = &t
	}

	rsv, err := h.svc.CreateReservation(c.Request.Context(), ReservationInput{
		ProductID:     pid,
		WarehouseID:   wid,
		Quantity:      req.Quantity,
		ReferenceType: req.ReferenceType,
		ReferenceID:   req.ReferenceID,
		ExpiresAt:     expires,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	details := gin.H{
		"reservation_id": rsv.ID.String(),
		"product_id":     pid.String(),
		"warehouse_id":   wid.String(),
		"quantity":       req.Quantity,
	}
	h.audit.Record(audit.EntryFromContext(c, audit.Entry{
		Action:     "RESERVATION_CREATED",
		EntityType: "reservation",
		EntityID:   &[]string{rsv.ID.String()}[0],
		Details:    details,
	}))
	response.Created(c, gin.H{
		"id":             rsv.ID,
		"product_id":     rsv.ProductID,
		"warehouse_id":   rsv.WarehouseID,
		"quantity":       rsv.Quantity,
		"reference_type": rsv.ReferenceType,
		"reference_id":   rsv.ReferenceID,
		"status":         rsv.Status,
	})
}

// ReleaseReservation handles POST /inventory/reservations/:id/release.
func (h *Handler) ReleaseReservation(c *gin.Context) {
	id, ok := parseUUID(c.Param("id"))
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	rsv, err := h.svc.ReleaseReservation(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(audit.EntryFromContext(c, audit.Entry{
		Action:     "RESERVATION_RELEASED",
		EntityType: "reservation",
		EntityID:   &[]string{rsv.ID.String()}[0],
		Details:    gin.H{"quantity": rsv.Quantity},
	}))
	response.OK(c, gin.H{"id": rsv.ID, "status": rsv.Status})
}

// ConsumeReservation handles POST /inventory/reservations/:id/consume —
// converts the reservation into an ISSUE ledger entry atomically.
func (h *Handler) ConsumeReservation(c *gin.Context) {
	id, ok := parseUUID(c.Param("id"))
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	userID := middleware.UserIDFromContext(c)
	rsv, inv, err := h.svc.ConsumeReservation(c.Request.Context(), id, &userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(audit.EntryFromContext(c, audit.Entry{
		Action:     "RESERVATION_CONSUMED",
		EntityType: "reservation",
		EntityID:   &[]string{rsv.ID.String()}[0],
		Details: gin.H{
			"quantity":   rsv.Quantity,
			"product_id": rsv.ProductID.String(),
		},
		BeforeData: gin.H{"quantity": inv.Quantity + rsv.Quantity},
		AfterData:  gin.H{"quantity": inv.Quantity},
	}))
	response.OK(c, gin.H{
		"id":          rsv.ID,
		"status":      rsv.Status,
		"quantity":    inv.Quantity,
		"reserved":    inv.ReservedQuantity,
		"available":   inv.Quantity - inv.ReservedQuantity,
		"updated_at":  inv.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

// ListReservations handles GET /inventory/reservations.
func (h *Handler) ListReservations(c *gin.Context) {
	var req reservationsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	q := ReservationQuery{Status: req.Status, Page: req.Page, PerPage: req.PerPage}
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

	views, total, err := h.svc.Reservations(c.Request.Context(), q)
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]reservationEnvelope, 0, len(views))
	for _, v := range views {
		items = append(items, reservationEnvelope{
			ID:            v.ID,
			ProductID:     v.ProductID,
			ProductSKU:    v.ProductSKU,
			ProductName:   v.ProductName,
			WarehouseID:   v.WarehouseID,
			Quantity:      v.Quantity,
			ReferenceType: v.ReferenceType,
			ReferenceID:   v.ReferenceID,
			Status:        v.Status,
			ExpiresAt:     v.ExpiresAt,
			CreatedAt:     v.CreatedAt,
		})
	}

	response.Paginated(c, items, &response.Pagination{
		Page:       clampPage(req.Page),
		PerPage:    clampPerPage(req.PerPage),
		Total:      total,
		TotalPages: int((total + int64(clampPerPage(req.PerPage)) - 1) / int64(clampPerPage(req.PerPage))),
	})
}
