// Auth service: register, login, refresh rotation, logout, password
// change, and profile update. Depends on a Repository abstraction so it
// is unit-testable with mocks.
package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	sharederr "inventory/internal/shared/errors"
)

// ErrEmailTaken is returned when registering an email that already exists.
var ErrEmailTaken = errors.New("email already registered")

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
	CreateUser(u *User) error
	FindUserByEmail(email string) (*User, error)
	FindUserByID(id uuid.UUID) (*User, error)
	UpdateUser(u *User) error

	FindRoleByName(name string) (*Role, error)
	FindRoleByID(id uuid.UUID) (*Role, error)

	CreateRefreshToken(t *RefreshToken) error
	FindRefreshTokenByHash(hash string) (*RefreshToken, error)
	UpdateRefreshToken(t *RefreshToken) error

	CreateActivityLog(entry ActivityLogEntry) error
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
}

// Register creates a STAFF user with a bcrypt-hashed password.
func (s *Service) Register(req RegisterRequest) (*User, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Name == "" || req.Password == "" {
		return nil, sharederr.ErrValidation
	}

	existing, err := s.repo.FindUserByEmail(email)
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

	role, err := s.repo.FindRoleByName("STAFF")
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
	if err := s.repo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// Login verifies credentials and issues an access + refresh token pair.
func (s *Service) Login(email, password string) (*AuthResult, error) {
	user, err := s.repo.FindUserByEmail(strings.ToLower(strings.TrimSpace(email)))
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

	res, err := s.issueTokens(user)
	if err != nil {
		return nil, err
	}
	s.logActivity(user.ID, "LOGIN", "user")
	return res, nil
}

// Refresh rotates a presented refresh token: revokes it and issues a new pair.
func (s *Service) Refresh(rawToken string) (*AuthResult, error) {
	hash := s.tokens.HashRefreshToken(rawToken)
	rt, err := s.repo.FindRefreshTokenByHash(hash)
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

	user, err := s.repo.FindUserByID(rt.UserID)
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
	if err := s.repo.UpdateRefreshToken(rt); err != nil {
		return nil, fmt.Errorf("revoke old refresh token: %w", err)
	}
	res, err := s.issueTokens(user)
	if err != nil {
		return nil, err
	}
	s.logActivity(user.ID, "REFRESH", "user")
	return res, nil
}

// Logout revokes the presented refresh token (idempotent).
func (s *Service) Logout(rawToken string) error {
	hash := s.tokens.HashRefreshToken(rawToken)
	rt, err := s.repo.FindRefreshTokenByHash(hash)
	if err != nil {
		if errors.Is(err, sharederr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("find refresh token: %w", err)
	}
	now := time.Now()
	rt.RevokedAt = &now
	if err := s.repo.UpdateRefreshToken(rt); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	s.logActivity(rt.UserID, "LOGOUT", "user")
	return nil
}

// ChangePassword verifies the old password and sets the new one.
func (s *Service) ChangePassword(userID uuid.UUID, oldPassword, newPassword string) error {
	user, err := s.repo.FindUserByID(userID)
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
	if err := s.repo.UpdateUser(user); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	s.logActivity(userID, "CHANGE_PASSWORD", "user")
	return nil
}

// UpdateProfile updates a user's name/email (email uniqueness checked).
func (s *Service) UpdateProfile(userID uuid.UUID, name, email string) (*User, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, err
	}
	if email != "" && email != user.Email {
		other, err := s.repo.FindUserByEmail(email)
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
	if err := s.repo.UpdateUser(user); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	s.logActivity(userID, "UPDATE_PROFILE", "user")
	return user, nil
}

func (s *Service) issueTokens(user *User) (*AuthResult, error) {
	role, err := s.repo.FindRoleByID(user.RoleID)
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
	if err := s.repo.CreateRefreshToken(rt); err != nil {
		return nil, fmt.Errorf("persist refresh token: %w", err)
	}
	return &AuthResult{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int64(s.tokens.accessTTL.Seconds()),
		User:         user,
	}, nil
}

func (s *Service) logActivity(userID uuid.UUID, action, entityType string) {
	entry := ActivityLogEntry{
		UserID:     &userID,
		Action:     action,
		EntityType: entityType,
	}
	_ = s.repo.CreateActivityLog(entry)
}
