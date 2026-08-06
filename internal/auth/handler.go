// HTTP handlers for the auth module. Each handler validates its DTO,
// calls the service, and writes the shared response envelope.
package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"inventory/internal/shared/audit"
	"inventory/internal/shared/errors"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/response"
	"inventory/internal/shared/validator"
)

// Handler exposes auth routes.
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
func (h *Handler) record(c *gin.Context, action, entityID string, userID *uuid.UUID, details gin.H) {
	eid := entityID
	ip := c.ClientIP()
	h.audit.Record(audit.Entry{
		UserID:     userID,
		Action:     action,
		EntityType: "user",
		EntityID:   &eid,
		Details:    details,
		IP:         &ip,
	})
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
// @Tags auth
// @Summary Register a new account
// @Accept json
// @Produce json
// @Param body body registerRequest true "Registration payload"
// @Success 201 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Router /auth/register [post]
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

	user, err := h.svc.Register(RegisterRequest(req))
	if err != nil {
		response.Error(c, err)
		return
	}
	role, _ := h.svc.RoleName(user.ID)
	h.record(c, "REGISTER", user.ID.String(), &user.ID, gin.H{"email": user.Email, "name": user.Name})
	response.Created(c, userResponse(user, role))
}

// Login handles POST /auth/login.
// @Tags auth
// @Summary Log in
// @Accept json
// @Produce json
// @Param body body loginRequest true "Login credentials"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /auth/login [post]
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
	if res.User != nil {
		h.record(c, "LOGIN", res.User.ID.String(), &res.User.ID, gin.H{"email": res.User.Email})
	}
	response.OK(c, loginResultResponse(res))
}

// DemoLogin handles POST /auth/demo. It returns tokens for the demo STAFF
// user, creating it on first use. Guarded externally by DEMO_MODE.
// @Tags auth
// @Summary Demo auto-login (requires demo mode)
// @Accept json
// @Produce json
// @Success 200 {object} response.Body
// @Failure 404 {object} response.Body "demo mode disabled"
// @Router /auth/demo [post]
func (h *Handler) DemoLogin(c *gin.Context) {
	res, err := h.svc.DemoLogin()
	if err != nil {
		response.Error(c, err)
		return
	}
	if res.User != nil {
		h.record(c, "LOGIN", res.User.ID.String(), &res.User.ID, gin.H{"email": res.User.Email})
	}
	response.OK(c, loginResultResponse(res))
}

// Refresh handles POST /auth/refresh.
// @Tags auth
// @Summary Refresh access token
// @Accept json
// @Produce json
// @Param body body refreshRequest true "Refresh token"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /auth/refresh [post]
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
// @Tags auth
// @Summary Log out
// @Accept json
// @Produce json
// @Param body body refreshRequest true "Refresh token"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /auth/logout [post]
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
// @Tags auth
// @Summary Change password
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body changePasswordRequest true "Password change payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /auth/change-password [post]
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
	h.record(c, "CHANGE_PASSWORD", userID.String(), &userID, nil)
	response.Message(c, "password changed")
}

// UpdateProfile handles PUT /auth/profile.
// @Tags auth
// @Summary Update own profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body updateProfileRequest true "Profile update payload"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /auth/profile [put]
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
	h.record(c, "UPDATE_PROFILE", user.ID.String(), &userID, gin.H{"name": user.Name, "email": user.Email})
	response.OK(c, userResponse(user, role))
}

// Me handles GET /auth/me.
// @Tags auth
// @Summary Get current user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Failure 401 {object} response.Body
// @Router /auth/me [get]
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
