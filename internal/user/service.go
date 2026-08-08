// Service logic for admin user management. Consumes a Repository
// interface (persistence-agnostic) and returns sentinel-wrapped errors.
package user

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
)

// Repository abstracts persistence for the user service.
type Repository interface {
	// List returns a page of users matching the filters and the total count
	// matching those filters (before paging).
	List(ctx context.Context, q Query) ([]*User, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, u *User) error
	FindRoleByName(ctx context.Context, name string) (*Role, error)
	CountAdmins(ctx context.Context) (int64, error)
}

// Service orchestrates admin user management.
type Service struct {
	repo Repository
}

// NewService wires a repository into the service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns a page of users plus the total match count.
func (s *Service) List(ctx context.Context, q Query) ([]*User, int64, error) {
	q.Name = strings.TrimSpace(q.Name)
	q.Email = strings.TrimSpace(q.Email)
	q.Role = strings.ToUpper(strings.TrimSpace(q.Role))
	return s.repo.List(ctx, q)
}

// Get returns a single user by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.FindByID(ctx, id)
}

// UpdateName changes the display name of a user.
func (s *Service) UpdateName(ctx context.Context, id uuid.UUID, name string) (*User, error) {
	if strings.TrimSpace(name) == "" {
		return nil, sharederr.ErrValidation
	}
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u.Name = strings.TrimSpace(name)
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// UpdateProfile updates a user's name, email, and/or active state as an
// admin. Empty fields are left unchanged. Email changes are guarded against
// collisions with another account. Deactivation applies the same guards as
// Deactivate (cannot deactivate self; cannot deactivate the last admin).
func (s *Service) UpdateProfile(ctx context.Context, id, actorID uuid.UUID, name, email string, isActive *bool) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if name != "" {
		if strings.TrimSpace(name) == "" {
			return nil, sharederr.ErrValidation
		}
		u.Name = strings.TrimSpace(name)
	}

	if email != "" {
		email = strings.ToLower(strings.TrimSpace(email))
		if email != u.Email {
			existing, err := s.repo.FindByEmail(ctx, email)
			if err == nil && existing.ID != id {
				return nil, sharederr.ErrConflict
			}
			if err != nil && !errors.Is(err, sharederr.ErrNotFound) {
				return nil, err
			}
			u.Email = email
		}
	}

	if isActive != nil {
		if !*isActive {
			if id == actorID {
				return nil, sharederr.ErrConflict // cannot deactivate self
			}
			role, err := s.repo.FindRoleByName(ctx, "ADMIN")
			if err != nil {
				return nil, err
			}
			if u.RoleID == role.ID {
				admins, err := s.repo.CountAdmins(ctx)
				if err != nil {
					return nil, err
				}
				if admins <= 1 {
					return nil, sharederr.ErrConflict // last admin
				}
			}
		}
		u.IsActive = *isActive
	}

	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// AssignRole sets the user's role by role name (ADMIN or STAFF).
func (s *Service) AssignRole(ctx context.Context, id uuid.UUID, roleName string) (*User, error) {
	role, err := s.repo.FindRoleByName(ctx, strings.ToUpper(strings.TrimSpace(roleName)))
	if err != nil {
		return nil, err
	}
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u.RoleID = role.ID
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Activate re-enables a deactivated user.
func (s *Service) Activate(ctx context.Context, id uuid.UUID) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u.IsActive = true
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Deactivate disables a user account. An admin cannot deactivate their own
// account, and the last active admin cannot be deactivated at all.
func (s *Service) Deactivate(ctx context.Context, id uuid.UUID, actorID uuid.UUID) (*User, error) {
	if id == actorID {
		return nil, sharederr.ErrConflict // cannot deactivate self
	}
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	role, err := s.repo.FindRoleByName(ctx, "ADMIN")
	if err != nil {
		return nil, err
	}
	if u.RoleID == role.ID {
		admins, err := s.repo.CountAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if admins <= 1 {
			return nil, sharederr.ErrConflict // last admin
		}
	}
	u.IsActive = false
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
