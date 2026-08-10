// HTTP handlers for warehouses. Authenticated read routes and admin-only
// write routes, all returning the shared response envelope.
package warehouses

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"inventory/internal/shared/audit"
	"inventory/internal/shared/dbutil"
	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/response"
	"inventory/internal/shared/validator"
)

// Handler exposes warehouse routes.
type Handler struct {
	svc   *Service
	val   *validator.Validator
	audit audit.Recorder
}

// NewHandler wires the service and validator.
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
		EntityType: "warehouse",
		EntityID:   &eid,
		Details:    details,
		IP:         &ip,
	})
}

type listWarehousesRequest struct {
	Page     int    `form:"page"`
	PerPage  int    `form:"per_page"`
	Search   string `form:"search"`
	IsActive string `form:"is_active"`
	Sort     string `form:"sort"`
}

type createWarehouseRequest struct {
	Code        string  `json:"code" validate:"required,min=2"`
	Name        string  `json:"name" validate:"required,min=2"`
	Description *string `json:"description"`
}

type updateWarehouseRequest struct {
	Code        *string `json:"code"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

type warehouseEnvelope struct {
	ID             uuid.UUID `json:"id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	Description    *string   `json:"description,omitempty"`
	IsActive       bool      `json:"is_active"`
	InventoryCount int64     `json:"inventory_count"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at"`
}

func warehouseResponse(w *Warehouse) warehouseEnvelope {
	return warehouseEnvelope{
		ID:             w.ID,
		Code:           w.Code,
		Name:           w.Name,
		Description:    w.Description,
		IsActive:       w.IsActive,
		InventoryCount: w.InventoryCount,
		CreatedAt:      w.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      w.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// List handles GET /warehouses — public, paginated/filtered/sorted.
// @Tags warehouses
// @Summary List warehouses
// @Accept json
// @Produce json
// @Param search query string false "Filter by name or code"
// @Param is_active query string false "Filter by active state" Enums(true,false)
// @Param sort query string false "Sort field/direction" Enums(name,code,created_at,-name,-code,-created_at)
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Router /warehouses [get]
func (h *Handler) List(c *gin.Context) {
	var req listWarehousesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	isActive := parseActiveFilter(req.IsActive)

	whs, total, err := h.svc.List(c.Request.Context(), ListQuery{
		Search:   req.Search,
		IsActive: isActive,
		Sort:     req.Sort,
		Page:     req.Page,
		PerPage:  req.PerPage,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]warehouseEnvelope, 0, len(whs))
	for _, w := range whs {
		items = append(items, warehouseResponse(w))
	}

	page, perPage := dbutil.NormalizePage(req.Page, req.PerPage)
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

// Get handles GET /warehouses/:id — public read.
// @Tags warehouses
// @Summary Get a warehouse
// @Accept json
// @Produce json
// @Param id path string true "Warehouse ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Router /warehouses/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	w, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, warehouseResponse(w))
}

// Create handles POST /warehouses — admin only.
// @Tags warehouses
// @Summary Create a warehouse
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createWarehouseRequest true "Warehouse payload"
// @Success 201 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /warehouses [post]
func (h *Handler) Create(c *gin.Context) {
	var req createWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	w, err := h.svc.Create(c.Request.Context(), req.Code, req.Name, req.Description)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "CREATE", w.ID.String(), gin.H{"code": w.Code, "name": w.Name})
	response.Created(c, warehouseResponse(w))
}

// Update handles PUT /warehouses/:id — admin only.
// @Tags warehouses
// @Summary Update a warehouse
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Warehouse ID"
// @Param body body updateWarehouseRequest true "Warehouse update payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /warehouses/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	var req updateWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if req.Code == nil && req.Name == nil && req.Description == nil && req.IsActive == nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	w, err := h.svc.Update(c.Request.Context(), id, UpdateParams(req))
	if err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "UPDATE", w.ID.String(), gin.H{"code": w.Code, "name": w.Name})
	response.OK(c, warehouseResponse(w))
}

// Delete handles DELETE /warehouses/:id — admin only. Fails with 409 when
// inventory rows still reference the warehouse.
// @Tags warehouses
// @Summary Delete a warehouse
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Warehouse ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /warehouses/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "DELETE", id.String(), gin.H{"deactivated": true})
	response.Message(c, "warehouse deactivated")
}

// parseActiveFilter converts an "is_active=true|false" query value into a
// *bool filter (nil when absent).
func parseActiveFilter(s string) *bool {
	var v *bool
	switch s {
	case "true":
		t := true
		v = &t
	case "false":
		f := false
		v = &f
	}
	return v
}
