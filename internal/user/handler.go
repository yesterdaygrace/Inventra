// HTTP handlers for admin user management. Each handler validates its DTO,
// resolves the acting admin from context, calls the service, and writes the
// shared response envelope.
package user

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

// Handler exposes admin user routes.
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
		EntityType: "user",
		EntityID:   &eid,
		Details:    details,
		IP:         &ip,
	})
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
// @Tags users
// @Summary List users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param name query string false "Filter by name"
// @Param email query string false "Filter by email"
// @Param role query string false "Filter by role" Enums(ADMIN, STAFF)
// @Param is_active query string false "Filter by active state" Enums(true, false)
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Router /users [get]
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

	users, total, err := h.svc.List(c.Request.Context(), Query{
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
// @Tags users
// @Summary Get a user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Router /users/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}
	u, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, userResponse(u))
}

// Update handles PUT /users/:id — admin-only.
// @Tags users
// @Summary Update a user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param body body updateUserRequest true "User update payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /users/{id} [put]
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
	u, err := h.svc.UpdateProfile(c.Request.Context(), id, actorID, req.Name, req.Email, req.IsActive)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "UPDATE", id.String(), gin.H{"name": u.Name, "email": u.Email})
	response.OK(c, userResponse(u))
}

// Deactivate handles DELETE /users/:id — admin-only soft deactivate.
// @Tags users
// @Summary Deactivate a user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /users/{id} [delete]
func (h *Handler) Deactivate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	actorID := middleware.UserIDFromContext(c)
	if _, err := h.svc.Deactivate(c.Request.Context(), id, actorID); err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "DEACTIVATE", id.String(), nil)
	response.Message(c, "user deactivated")
}

// AssignRole handles PUT /users/:id/role — admin-only.
// @Tags users
// @Summary Assign a role to a user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param body body assignRoleRequest true "Role assignment payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 403 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /users/{id}/role [put]
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

	u, err := h.svc.AssignRole(c.Request.Context(), id, req.Role)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.record(c, "ASSIGN_ROLE", id.String(), gin.H{"role": req.Role})
	response.OK(c, userResponse(u))
}
