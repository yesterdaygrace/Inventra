// HTTP handlers for the auth module. Each handler validates its DTO,
// calls the service, and writes the shared response envelope.
package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"inventory/internal/shared/errors"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/response"
	"inventory/internal/shared/validator"
)

// Handler exposes auth routes.
type Handler struct {
	svc *Service
	val *validator.Validator
}

// NewHandler wires the service and validator.
func NewHandler(svc *Service, val *validator.Validator) *Handler {
	return &Handler{svc: svc, val: val}
}

type registerRequest struct {
	Name     string `json:"name" validate:"required,min=2"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type updateProfileRequest struct {
	Name  string `json:"name" validate:"min=2"`
	Email string `json:"email" validate:"omitempty,email"`
}

// Register handles POST /auth/register.
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}

	user, err := h.svc.Register(RegisterRequest{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	role, _ := h.svc.RoleName(user.ID)
	response.Created(c, userResponse(user, role))
}

// Login handles POST /auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}

	res, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, loginResultResponse(res))
}

// Refresh handles POST /auth/refresh.
func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}

	res, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, loginResultResponse(res))
}

// Logout handles POST /auth/logout.
func (h *Handler) Logout(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}

	if err := h.svc.Logout(req.RefreshToken); err != nil {
		response.Error(c, err)
		return
	}
	response.Message(c, "logged out")
}

// ChangePassword handles POST /auth/change-password.
func (h *Handler) ChangePassword(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	if userID == uuid.Nil {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}

	if err := h.svc.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		response.Error(c, err)
		return
	}
	response.Message(c, "password changed")
}

// UpdateProfile handles PUT /auth/profile.
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	if userID == uuid.Nil {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}
	if err := h.val.Validate(req); err != nil {
		response.Error(c, errors.ErrValidation)
		return
	}

	user, err := h.svc.UpdateProfile(userID, req.Name, req.Email)
	if err != nil {
		response.Error(c, err)
		return
	}
	role, _ := h.svc.RoleName(user.ID)
	response.OK(c, userResponse(user, role))
}

// Me handles GET /auth/me.
func (h *Handler) Me(c *gin.Context) {
	userID := middleware.UserIDFromContext(c)
	if userID == uuid.Nil {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	user, role, err := h.svc.Profile(userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, userResponse(user, role))
}

type userEnvelope struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	IsActive bool      `json:"is_active"`
}

func userResponse(u *User, role string) userEnvelope {
	return userEnvelope{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		Role:     role,
		IsActive: u.IsActive,
	}
}

type loginEnvelope struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int64        `json:"expires_in"`
	User         userEnvelope `json:"user"`
}

func loginResultResponse(res *AuthResult) loginEnvelope {
	var role string
	if res.User != nil {
		role = res.RoleName
	}
	return loginEnvelope{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    res.ExpiresIn,
		User:         userResponse(res.User, role),
	}
}
