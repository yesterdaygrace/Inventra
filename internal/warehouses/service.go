// Service logic for warehouses. Consumes a Repository interface and returns
// sentinel-wrapped errors.
package warehouses

import (
	"context"
	"strings"

	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
)

// ListQuery carries list filters, sort, and pagination.
type ListQuery struct {
	Search   string // substring match on name or code (case-insensitive)
	IsActive *bool
	Sort     string // "name", "code" or "created_at" (default "name"); "-" prefix = desc
	Page     int
	PerPage  int
}

// Repository abstracts persistence for the warehouse service.
type Repository interface {
	Create(ctx context.Context, w *Warehouse) error
	Get(ctx context.Context, id uuid.UUID) (*Warehouse, error)
	GetByCode(ctx context.Context, code string) (*Warehouse, error)
	Update(ctx context.Context, w *Warehouse) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, q ListQuery) ([]*Warehouse, int64, error)
	CountInventoryFor(ctx context.Context, id uuid.UUID) (int64, error)
	ListWithInventoryCount(ctx context.Context, q ListQuery) ([]*Warehouse, int64, error)
}

// Service orchestrates warehouse management.
type Service struct {
	repo Repository
}

// NewService wires a repository into the service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new warehouse.
func (s *Service) Create(ctx context.Context, code, name string, description *string) (*Warehouse, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, sharederr.ErrValidation
	}
	w := &Warehouse{Code: code, Name: name, Description: trimDescription(description)}
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

// Get returns a warehouse by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Warehouse, error) {
	return s.repo.Get(ctx, id)
}

// GetByCode returns a warehouse by its code string.
func (s *Service) GetByCode(ctx context.Context, code string) (*Warehouse, error) {
	return s.repo.GetByCode(ctx, code)
}

// UpdateParams carries the subset of fields a caller wants to change on a
// warehouse. Nil pointers mean "leave unchanged"; non-nil values overwrite.
type UpdateParams struct {
	Code        *string
	Name        *string
	Description *string
	IsActive    *bool
}

// Update changes a warehouse, overwriting only the fields present in params.
func (s *Service) Update(ctx context.Context, id uuid.UUID, params UpdateParams) (*Warehouse, error) {
	if params.Code != nil && strings.TrimSpace(*params.Code) == "" {
		return nil, sharederr.ErrValidation
	}
	if params.Name != nil && strings.TrimSpace(*params.Name) == "" {
		return nil, sharederr.ErrValidation
	}

	w, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.Code != nil {
		w.Code = strings.TrimSpace(*params.Code)
	}
	if params.Name != nil {
		w.Name = strings.TrimSpace(*params.Name)
	}
	if params.Description != nil {
		w.Description = trimDescription(params.Description)
	}
	if params.IsActive != nil {
		w.IsActive = *params.IsActive
	}

	if err := s.repo.Update(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

// Delete deactivates a warehouse provided no inventory references it.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	count, err := s.repo.CountInventoryFor(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return sharederr.ErrConflict
	}
	return s.repo.Delete(ctx, id)
}

// List is a passthrough to the repository list enriched with inventory counts.
func (s *Service) List(ctx context.Context, q ListQuery) ([]*Warehouse, int64, error) {
	q.Search = strings.TrimSpace(q.Search)
	return s.repo.ListWithInventoryCount(ctx, q)
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
