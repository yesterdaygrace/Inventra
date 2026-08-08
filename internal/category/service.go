// Service logic for product categories. Consumes a Repository interface
// and returns sentinel-wrapped errors.
package category

import (
	"context"
	"strings"

	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
)

// ListQuery carries list filters, sort, and pagination.
type ListQuery struct {
	Search   string // substring match on name (case-insensitive)
	IsActive *bool
	Sort     string // "name" or "created_at" (default "name"); "-" prefix = desc
	Page     int
	PerPage  int
}

// Repository abstracts persistence for the category service.
type Repository interface {
	Create(ctx context.Context, c *Category) error
	Get(ctx context.Context, id uuid.UUID) (*Category, error)
	Update(ctx context.Context, c *Category) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, q ListQuery) ([]*Category, int64, error)
	CountProductsFor(ctx context.Context, id uuid.UUID) (int64, error)
}

// Service orchestrates category management.
type Service struct {
	repo Repository
}

// NewService wires a repository into the service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new category.
func (s *Service) Create(ctx context.Context, name string, description *string) (*Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, sharederr.ErrValidation
	}
	c := &Category{Name: name, Description: trimDescription(description)}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Get returns a category by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Category, error) {
	return s.repo.Get(ctx, id)
}

// Update changes a category name/description/active flag.
func (s *Service) Update(ctx context.Context, id uuid.UUID, name string, description *string, isActive *bool) (*Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, sharederr.ErrValidation
	}
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	c.Name = name
	c.Description = trimDescription(description)
	if isActive != nil {
		c.IsActive = *isActive
	}
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Delete removes a category provided no product references it.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	count, err := s.repo.CountProductsFor(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return sharederr.ErrConflict
	}
	return s.repo.Delete(ctx, id)
}

// List is a pass-through to the repository list.
func (s *Service) List(ctx context.Context, q ListQuery) ([]*Category, int64, error) {
	q.Search = strings.TrimSpace(q.Search)
	return s.repo.List(ctx, q)
}

func trimDescription(d *string) *string {
	if d == nil {
		return nil
	}
	s := strings.TrimSpace(*d)
	if s == "" {
		return nil
	}
	return &s
}
