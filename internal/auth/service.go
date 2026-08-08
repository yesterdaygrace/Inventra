// Auth service: register, login, refresh rotation, logout, password
// change, and profile update. Depends on a Repository abstraction so it
// is unit-testable with mocks.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	sharederr "inventory/internal/shared/errors"
)

// ErrEmailTaken is returned when registering an email that already exists.
// It wraps ErrConflict so the response envelope maps it to HTTP 409 while
// callers can still match it directly with errors.Is.
var ErrEmailTaken = fmt.Errorf("%w: email already registered", sharederr.ErrConflict)

// ActivityLogEntry is a minimal audit event recorded by the service. It is
// intentionally decoupled from the activitylog package's model to avoid an
// import cycle (activitylog imports auth.User); the concrete repository that
// persists it imports activitylog instead.
type ActivityLogEntry struct {
	UserID     *uuid.UUID
	Action     string
	EntityType string
}

// Repository abstracts persistence for the auth service.
type Repository interface {
	CreateUser(ctx context.Context, u *User) error
	FindUserByEmail(ctx context.Context, email string) (*User, error)
	FindUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateUser(ctx context.Context, u *User) error

	FindRoleByName(ctx context.Context, name string) (*Role, error)
	FindRoleByID(ctx context.Context, id uuid.UUID) (*Role, error)

	CreateRefreshToken(ctx context.Context, t *RefreshToken) error
	FindRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error)
	UpdateRefreshToken(ctx context.Context, t *RefreshToken) error

	CreateActivityLog(ctx context.Context, entry ActivityLogEntry) error
}

// Service orchestrates authentication flows.
type Service struct {
	repo   Repository
	tokens *TokenManager
	cost   int
}

// NewService wires a repository, token manager, and bcrypt cost.
func NewService(repo Repository, tokens *TokenManager, bcryptCost int) *Service {
	return &Service{repo: repo, tokens: tokens, cost: bcryptCost}
}

// RegisterRequest is the input for creating an account.
type RegisterRequest struct {
	Name     string
	Email    string
	Password string
}

// AuthResult is the output of login and refresh.
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	User         *User
	RoleName     string
}

// Profile returns a user with its role name resolved.
func (s *Service) Profile(ctx context.Context, userID uuid.UUID) (*User, string, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	role, err := s.RoleName(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	return user, role, nil
}

// Register creates a STAFF user with a bcrypt-hashed password.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*User, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Name == "" || req.Password == "" {
		return nil, sharederr.ErrValidation
	}

	existing, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, sharederr.ErrNotFound) {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.cost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	role, err := s.repo.FindRoleByName(ctx, "STAFF")
	if err != nil {
		return nil, fmt.Errorf("find STAFF role: %w", err)
	}

	user := &User{
		Name:         req.Name,
		Email:        email,
		PasswordHash: string(hash),
		RoleID:       role.ID,
		IsActive:     true,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// Login verifies credentials and issues an access + refresh token pair.
func (s *Service) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	user, err := s.repo.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, sharederr.ErrNotFound) {
			return nil, sharederr.ErrUnauthorized
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	if !user.IsActive {
		return nil, sharederr.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, sharederr.ErrUnauthorized
	}

	res, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
	}
	s.logActivity(ctx, user.ID, "LOGIN", "user")
	return res, nil
}

// DemoEmail is the fixed identity used by demo auto-login mode. It is
// deterministic so the demo user is created/loaded idempotently.
const DemoEmail = "demo@inventory.local"

// DemoLogin returns a token pair for the demo STAFF user, creating the
// account on first call if it does not exist yet. It never requires a
// password: access is granted purely by the endpoint being reachable
// (guarded externally by DEMO_MODE). The demo password hash is a fresh
// random bcrypt value so the account has no known credential.
func (s *Service) DemoLogin(ctx context.Context) (*AuthResult, error) {
	user, err := s.repo.FindUserByEmail(ctx, DemoEmail)
	switch {
	case err == nil && user != nil:
		// existing demo user: issue tokens directly
	case errors.Is(err, sharederr.ErrNotFound):
		role, rerr := s.repo.FindRoleByName(ctx, "STAFF")
		if rerr != nil {
			return nil, fmt.Errorf("find STAFF role: %w", rerr)
		}
		randomPass, perr := randomPasswordHash(s.cost)
		if perr != nil {
			return nil, perr
		}
		user = &User{
			Name:         "Demo User",
			Email:        DemoEmail,
			PasswordHash: randomPass,
			RoleID:       role.ID,
			IsActive:     true,
		}
		if cerr := s.repo.CreateUser(ctx, user); cerr != nil {
			return nil, fmt.Errorf("create demo user: %w", cerr)
		}
	default:
		return nil, fmt.Errorf("find demo user: %w", err)
	}

	res, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
	}
	s.logActivity(ctx, user.ID, "LOGIN", "user")
	return res, nil
}

// randomPasswordHash generates an unusable bcrypt hash of a random password
// so the demo account has no known credential (demo login bypasses passwords).
func randomPasswordHash(cost int) (string, error) {
	raw := uuid.NewString()
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), cost)
	if err != nil {
		return "", fmt.Errorf("hash demo password: %w", err)
	}
	return string(hash), nil
}
func (s *Service) Refresh(ctx context.Context, rawToken string) (*AuthResult, error) {
	hash := s.tokens.HashRefreshToken(rawToken)
	rt, err := s.repo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sharederr.ErrNotFound) {
			return nil, sharederr.ErrUnauthorized
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	if rt.RevokedAt != nil {
		return nil, sharederr.ErrUnauthorized
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, sharederr.ErrUnauthorized
	}

	user, err := s.repo.FindUserByID(ctx, rt.UserID)
	if err != nil {
		if errors.Is(err, sharederr.ErrNotFound) {
			return nil, sharederr.ErrUnauthorized
		}
		return nil, fmt.Errorf("find refresh user: %w", err)
	}
	if !user.IsActive {
		return nil, sharederr.ErrUnauthorized
	}

	now := time.Now()
	rt.RevokedAt = &now
	if err := s.repo.UpdateRefreshToken(ctx, rt); err != nil {
		return nil, fmt.Errorf("revoke old refresh token: %w", err)
	}
	res, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
	}
	s.logActivity(ctx, user.ID, "REFRESH", "user")
	return res, nil
}

// Logout revokes the presented refresh token (idempotent).
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	hash := s.tokens.HashRefreshToken(rawToken)
	rt, err := s.repo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sharederr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("find refresh token: %w", err)
	}
	now := time.Now()
	rt.RevokedAt = &now
	if err := s.repo.UpdateRefreshToken(ctx, rt); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	s.logActivity(ctx, rt.UserID, "LOGOUT", "user")
	return nil
}

// ChangePassword verifies the old password and sets the new one.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return sharederr.ErrUnauthorized
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.cost)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	user.PasswordHash = string(hash)
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	s.logActivity(ctx, userID, "CHANGE_PASSWORD", "user")
	return nil
}

// UpdateProfile updates a user's name/email (email uniqueness checked).
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, name, email string) (*User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if email != "" && email != user.Email {
		other, err := s.repo.FindUserByEmail(ctx, email)
		if err == nil && other != nil && other.ID != userID {
			return nil, ErrEmailTaken
		}
		if err != nil && !errors.Is(err, sharederr.ErrNotFound) {
			return nil, fmt.Errorf("check email: %w", err)
		}
		user.Email = email
	}
	if name != "" {
		user.Name = name
	}
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	s.logActivity(ctx, userID, "UPDATE_PROFILE", "user")
	return user, nil
}

// RoleName returns the role name for a user.
func (s *Service) RoleName(ctx context.Context, userID uuid.UUID) (string, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	role, err := s.repo.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		return "", err
	}
	return role.Name, nil
}

func (s *Service) issueTokens(ctx context.Context, user *User) (*AuthResult, error) {
	role, err := s.repo.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("find role: %w", err)
	}
	access, err := s.tokens.SignAccessToken(user.ID, role.Name)
	if err != nil {
		return nil, err
	}
	raw, hash, expiresAt, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	rt := &RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	}
	if err := s.repo.CreateRefreshToken(ctx, rt); err != nil {
		return nil, fmt.Errorf("persist refresh token: %w", err)
	}
	return &AuthResult{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int64(s.tokens.accessTTL.Seconds()),
		User:         user,
		RoleName:     role.Name,
	}, nil
}

func (s *Service) logActivity(ctx context.Context, userID uuid.UUID, action, entityType string) {
	entry := ActivityLogEntry{
		UserID:     &userID,
		Action:     action,
		EntityType: entityType,
	}
	_ = s.repo.CreateActivityLog(ctx, entry)
}
