// HTTP handlers for product categories. Public read routes and
// admin-only write routes, all returning the shared response envelope.
package category

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/export"
	"inventory/internal/shared/response"
	"inventory/internal/shared/validator"
)

// Handler exposes category routes.
type Handler struct {
	svc *Service
	val *validator.Validator
}

// NewHandler wires the service and validator.
func NewHandler(svc *Service, val *validator.Validator) *Handler {
	return &Handler{svc: svc, val: val}
}

type listCategoriesRequest struct {
	Page    int    `form:"page"`
	PerPage int    `form:"per_page"`
	Name    string `form:"name"`
	Sort    string `form:"sort"`
}

type createCategoryRequest struct {
	Name        string  `json:"name" validate:"required,min=2"`
	Description *string `json:"description"`
}

type updateCategoryRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type categoryEnvelope struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

func categoryResponse(c *Category) categoryEnvelope {
	return categoryEnvelope{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		CreatedAt:   c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   c.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// List handles GET /categories — public, paginated/filtered/sorted.
func (h *Handler) List(c *gin.Context) {
	var req listCategoriesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	cats, total, err := h.svc.List(ListQuery{
		Search:  req.Name,
		Sort:    req.Sort,
		Page:    req.Page,
		PerPage: req.PerPage,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]categoryEnvelope, 0, len(cats))
	for _, cat := range cats {
		items = append(items, categoryResponse(cat))
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	perPage := req.PerPage
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

// Get handles GET /categories/:id — public read.
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	cat, err := h.svc.Get(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, categoryResponse(cat))
}

// Create handles POST /categories — admin only.
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

	cat, err := h.svc.Create(req.Name, req.Description)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, categoryResponse(cat))
}

// Update handles PUT /categories/:id — admin only.
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
	if strings.TrimSpace(req.Name) == "" && req.Description == nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	cat, err := h.svc.Update(id, req.Name, req.Description)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, categoryResponse(cat))
}

// Delete handles DELETE /categories/:id — admin only. Fails with 409 when
// products still reference the category.
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.svc.Delete(id); err != nil {
		response.Error(c, err)
		return
	}
	response.Message(c, "category deleted")
}

// Export handles GET /categories/export — CSV download.
func (h *Handler) Export(c *gin.Context) {
	cats, _, err := h.svc.List(ListQuery{PerPage: 1000})
	if err != nil {
		response.Error(c, err)
		return
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
