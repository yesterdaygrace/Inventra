// HTTP handlers for admin user management. Each handler validates its DTO,
// resolves the acting admin from context, calls the service, and writes the
// shared response envelope.
package user

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/response"
	"inventory/internal/shared/validator"
)

// Handler exposes admin user routes.
type Handler struct {
	svc *Service
	val *validator.Validator
}

// NewHandler wires the service and validator.
func NewHandler(svc *Service, val *validator.Validator) *Handler {
	return &Handler{svc: svc, val: val}
}

type listUsersRequest struct {
	Page     int    `form:"page"`
	PerPage  int    `form:"per_page"`
	Name     string `form:"name"`
	Email    string `form:"email"`
	Role     string `form:"role"`
	IsActive string `form:"is_active"`
}

type updateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	IsActive *bool  `json:"is_active"`
}

type assignRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=ADMIN STAFF"`
}

type userEnvelope struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

func userResponse(u *User) userEnvelope {
	role := ""
	if u.Role.Name != "" {
		role = u.Role.Name
	}
	return userEnvelope{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Role:      role,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: u.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// List handles GET /users — admin-only, paginated/filtered.
func (h *Handler) List(c *gin.Context) {
	var req listUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	var isActive *bool
	switch strings.ToLower(req.IsActive) {
	case "true":
		b := true
		isActive = &b
	case "false":
		b := false
		isActive = &b
	case "":
	default:
		response.Error(c, sharederr.ErrValidation)
		return
	}

	users, total, err := h.svc.List(Query{
		Name:     req.Name,
		Email:    req.Email,
		Role:     req.Role,
		IsActive: isActive,
		Page:     req.Page,
		PerPage:  req.PerPage,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]userEnvelope, 0, len(users))
	for _, u := range users {
		items = append(items, userResponse(u))
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

// Get handles GET /users/:id — admin-only.
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	u, err := h.svc.Get(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, userResponse(u))
}

// Update handles PUT /users/:id — admin-only.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if req.Name == "" && req.Email == "" && req.IsActive == nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	actorID := middleware.UserIDFromContext(c)
	u, err := h.svc.UpdateProfile(id, actorID, req.Name, req.Email, req.IsActive)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, userResponse(u))
}

// Deactivate handles DELETE /users/:id — admin-only soft deactivate.
func (h *Handler) Deactivate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	actorID := middleware.UserIDFromContext(c)
	if _, err := h.svc.Deactivate(id, actorID); err != nil {
		response.Error(c, err)
		return
	}
	response.Message(c, "user deactivated")
}

// AssignRole handles PUT /users/:id/role — admin-only.
func (h *Handler) AssignRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	var req assignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	u, err := h.svc.AssignRole(id, req.Role)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, userResponse(u))
}
