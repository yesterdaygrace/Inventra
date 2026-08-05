// HTTP handlers for products. Public read routes (list/get/export) and
// admin-only write routes, all returning the shared response envelope.
package product

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

// Handler exposes product routes.
type Handler struct {
	svc  *Service
	val  *validator.Validator
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
		EntityType: "product",
		EntityID:   &eid,
		Details:    details,
		IP:         &ip,
	})
}

type listProductsRequest struct {
	Q          string   `form:"q"`
	CategoryID string   `form:"category_id"`
	MinPrice   *float64 `form:"min_price"`
	MaxPrice   *float64 `form:"max_price"`
	LowStock   bool     `form:"low_stock"`
	Archived   string   `form:"is_archived"`
	Sort       string   `form:"sort"`
	Page       int      `form:"page"`
	PerPage    int      `form:"per_page"`
}

type createProductRequest struct {
	Name              string  `json:"name" validate:"required,min=2"`
	SKU               string  `json:"sku" validate:"required,min=2"`
	Description       *string `json:"description"`
	Price             float64 `json:"price"`
	CategoryID        string  `json:"category_id" validate:"required"`
	LowStockThreshold int     `json:"low_stock_threshold"`
	IsArchived        bool    `json:"is_archived"`
}

type updateProductRequest struct {
	Name              string  `json:"name"`
	SKU               string  `json:"sku"`
	Description       *string `json:"description"`
	Price             float64 `json:"price"`
	CategoryID        string  `json:"category_id"`
	LowStockThreshold int     `json:"low_stock_threshold"`
	IsArchived        bool    `json:"is_archived"`
}

// productEnvelope is the JSON shape returned by the API. It includes the
// category name alongside the id for client convenience.
type productEnvelope struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	SKU               string    `json:"sku"`
	Description       *string   `json:"description,omitempty"`
	Price             float64   `json:"price"`
	CategoryID        uuid.UUID `json:"category_id"`
	CategoryName      string    `json:"category_name"`
	LowStockThreshold int       `json:"low_stock_threshold"`
	IsArchived        bool      `json:"is_archived"`
	CreatedAt         string    `json:"created_at"`
	UpdatedAt         string    `json:"updated_at"`
}

func productResponse(p *Product) productEnvelope {
	return productEnvelope{
		ID:                p.ID,
		Name:              p.Name,
		SKU:               p.SKU,
		Description:       p.Description,
		Price:             p.Price,
		CategoryID:        p.CategoryID,
		CategoryName:      p.Category.Name,
		LowStockThreshold: p.LowStockThreshold,
		IsArchived:        p.IsArchived,
		CreatedAt:         p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func parseCategoryID(raw string) (uuid.UUID, bool) {
	if strings.TrimSpace(raw) == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

func toBoolPtr(v string) *bool {
	if v == "" {
		return nil
	}
	b := v == "true" || v == "1"
	return &b
}

// List handles GET /products.
func (h *Handler) List(c *gin.Context) {
	var req listProductsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	var catID uuid.UUID
	if req.CategoryID != "" {
		id, ok := parseCategoryID(req.CategoryID)
		if !ok {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		catID = id
	}

	prods, total, err := h.svc.List(ListQuery{
		Q:          req.Q,
		CategoryID: catID,
		MinPrice:   req.MinPrice,
		MaxPrice:   req.MaxPrice,
		LowStock:   req.LowStock,
		IsArchived: toBoolPtr(req.Archived),
		Sort:       req.Sort,
		Page:       req.Page,
		PerPage:    req.PerPage,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]productEnvelope, 0, len(prods))
	for _, p := range prods {
		items = append(items, productResponse(p))
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

// Get handles GET /products/:id.
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	p, err := h.svc.Get(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, productResponse(p))
}

// Create handles POST /products.
func (h *Handler) Create(c *gin.Context) {
	var req createProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	catID, ok := parseCategoryID(req.CategoryID)
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	p, err := h.svc.Create(req.Name, req.SKU, req.Description, req.Price, catID, req.LowStockThreshold, req.IsArchived)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "CREATE", p.ID.String(), gin.H{"name": p.Name, "sku": p.SKU})
	response.Created(c, productResponse(p))
}

// Update handles PUT /products/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	var req updateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	catID, ok := parseCategoryID(req.CategoryID)
	if !ok {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	p, err := h.svc.Update(id, req.Name, req.SKU, req.Description, req.Price, catID, req.LowStockThreshold, req.IsArchived)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "UPDATE", p.ID.String(), gin.H{"name": p.Name, "sku": p.SKU})
	response.OK(c, productResponse(p))
}

// Delete handles DELETE /products/:id.
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
	h.record(c, "DELETE", id.String(), nil)
	response.Message(c, "product deleted")
}

// Export handles GET /products/export — CSV download of all products.
func (h *Handler) Export(c *gin.Context) {
	prods, _, err := h.svc.List(ListQuery{PerPage: 1000})
	if err != nil {
		response.Error(c, err)
		return
	}

	rows := make([][]string, 0, len(prods))
	for _, p := range prods {
		desc := ""
		if p.Description != nil {
			desc = *p.Description
		}
		rows = append(rows, []string{
			p.ID.String(),
			p.Name,
			p.SKU,
			strconv.FormatFloat(p.Price, 'f', 2, 64),
			p.Category.Name,
			strconv.Itoa(p.LowStockThreshold),
			strconv.FormatBool(p.IsArchived),
			desc,
			p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	export.SetAttachment(c, "products")
	_ = export.WriteCSV(c.Writer, []string{"id", "name", "sku", "price", "category_name", "low_stock_threshold", "is_archived", "description", "created_at"}, rows)
}
