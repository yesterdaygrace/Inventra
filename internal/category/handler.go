// HTTP handlers for product categories. Public read routes and
// admin-only write routes, all returning the shared response envelope.
package category

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"inventory/internal/shared/audit"
	"inventory/internal/shared/dbutil"
	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/export"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/response"
	"inventory/internal/shared/validator"
)

// Handler exposes category routes.
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
		EntityType: "category",
		EntityID:   &eid,
		Details:    details,
		IP:         &ip,
	})
}

type listCategoriesRequest struct {
	Page     int    `form:"page"`
	PerPage  int    `form:"per_page"`
	Name     string `form:"name"`
	IsActive string `form:"is_active"`
	Sort     string `form:"sort"`
}

type createCategoryRequest struct {
	Name        string  `json:"name" validate:"required,min=2"`
	Description *string `json:"description"`
}

type updateCategoryRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

type categoryEnvelope struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description,omitempty"`
	IsActive     bool      `json:"is_active"`
	ProductCount int64     `json:"product_count"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

func categoryResponse(c *Category) categoryEnvelope {
	return categoryEnvelope{
		ID:           c.ID,
		Name:         c.Name,
		Description:  c.Description,
		IsActive:     c.IsActive,
		ProductCount: c.ProductCount,
		CreatedAt:    c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    c.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// List handles GET /categories — public, paginated/filtered/sorted.
// @Tags categories
// @Summary List categories
// @Accept json
// @Produce json
// @Param name query string false "Filter by name"
// @Param sort query string false "Sort field/direction" Enums(name,created_at,-name,-created_at)
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Router /categories [get]
func (h *Handler) List(c *gin.Context) {
	var req listCategoriesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	isActive := parseActiveFilter(req.IsActive)

	cats, total, err := h.svc.List(c.Request.Context(), ListQuery{
		Search:   req.Name,
		IsActive: isActive,
		Sort:     req.Sort,
		Page:     req.Page,
		PerPage:  req.PerPage,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]categoryEnvelope, 0, len(cats))
	for _, cat := range cats {
		items = append(items, categoryResponse(cat))
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

// Get handles GET /categories/:id — public read.
// @Tags categories
// @Summary Get a category
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Router /categories/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	cat, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, categoryResponse(cat))
}

// Create handles POST /categories — admin only.
// @Tags categories
// @Summary Create a category
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createCategoryRequest true "Category payload"
// @Success 201 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /categories [post]
func (h *Handler) Create(c *gin.Context) {
	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	cat, err := h.svc.Create(c.Request.Context(), req.Name, req.Description)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "CREATE", cat.ID.String(), gin.H{"name": cat.Name})
	response.Created(c, categoryResponse(cat))
}

// Update handles PUT /categories/:id — admin only.
// @Tags categories
// @Summary Update a category
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Param body body updateCategoryRequest true "Category update payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /categories/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	var req updateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if strings.TrimSpace(req.Name) == "" && req.Description == nil && req.IsActive == nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	cat, err := h.svc.Update(c.Request.Context(), id, req.Name, req.Description, req.IsActive)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "UPDATE", cat.ID.String(), gin.H{"name": cat.Name})
	response.OK(c, categoryResponse(cat))
}

// Delete handles DELETE /categories/:id — admin only. Fails with 409 when
// products still reference the category.
// @Tags categories
// @Summary Delete a category
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /categories/{id} [delete]
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
	response.Message(c, "category deactivated")
}

// Export handles GET /categories/export — CSV download.
// @Tags categories
// @Summary Export categories as CSV
// @Accept json
// @Produce text/csv
// @Success 200
// @Router /categories/export [get]
func (h *Handler) Export(c *gin.Context) {
	var cats []*Category
	for page := 1; ; page++ {
		pageCats, total, err := h.svc.List(c.Request.Context(), ListQuery{Page: page, PerPage: 100})
		if err != nil {
			response.Error(c, err)
			return
		}
		cats = append(cats, pageCats...)
		if len(pageCats) == 0 || len(cats) >= int(total) {
			break
		}
	}

	rows := make([][]string, 0, len(cats))
	for _, cat := range cats {
		desc := ""
		if cat.Description != nil {
			desc = *cat.Description
		}
		rows = append(rows, []string{cat.ID.String(), cat.Name, desc, cat.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")})
	}

	export.SetAttachment(c, "categories")
	if err := export.WriteCSV(c.Writer, []string{"id", "name", "description", "created_at"}, rows); err != nil {
		return
	}
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
