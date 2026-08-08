// Service logic for products. Consumes a Repository interface and
// returns sentinel-wrapped errors.
package product

import (
	"context"
	"strings"

	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
)

// ListQuery carries product list filters, sort, and pagination.
type ListQuery struct {
	Q          string // substring match on name or sku (case-insensitive)
	CategoryID uuid.UUID
	MinPrice   *float64
	MaxPrice   *float64
	LowStock   bool   // only products whose stock <= low-stock threshold
	IsArchived *bool  // nil = no filter
	Sort       string // whitelisted column; "-" prefix = desc
	Page       int
	PerPage    int
}

// Repository abstracts persistence for the product service.
type Repository interface {
	Create(ctx context.Context, p *Product) error
	Get(ctx context.Context, id uuid.UUID) (*Product, error)
	Update(ctx context.Context, p *Product) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, q ListQuery) ([]*Product, int64, error)
	SKUExists(ctx context.Context, sku string, excludeID uuid.UUID) (bool, error)
}

// Service orchestrates product management.
type Service struct {
	repo Repository
}

// NewService wires a repository into the service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new product, enforcing a unique SKU.
func (s *Service) Create(ctx context.Context, name, sku string, description *string, price float64,
	categoryID uuid.UUID, lowStockThreshold int, isArchived bool) (*Product, error) {

	name = strings.TrimSpace(name)
	sku = strings.TrimSpace(sku)
	if name == "" || sku == "" || price < 0 || lowStockThreshold < 0 {
		return nil, sharederr.ErrValidation
	}

	exists, err := s.repo.SKUExists(ctx, sku, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, sharederr.ErrConflict
	}

	p := &Product{
		Name:              name,
		SKU:               sku,
		Description:       trimDescription(description),
		Price:             price,
		CategoryID:        categoryID,
		LowStockThreshold: lowStockThreshold,
		IsArchived:        isArchived,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get returns a product by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Product, error) {
	return s.repo.Get(ctx, id)
}

// Update changes a product, enforcing unique SKU except for the row itself.
func (s *Service) Update(ctx context.Context, id uuid.UUID, name, sku string, description *string, price float64,
	categoryID uuid.UUID, lowStockThreshold int, isArchived bool) (*Product, error) {

	name = strings.TrimSpace(name)
	sku = strings.TrimSpace(sku)
	if name == "" || sku == "" || price < 0 || lowStockThreshold < 0 {
		return nil, sharederr.ErrValidation
	}

	exists, err := s.repo.SKUExists(ctx, sku, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, sharederr.ErrConflict
	}

	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Name = name
	p.SKU = sku
	p.Description = trimDescription(description)
	p.Price = price
	p.CategoryID = categoryID
	p.LowStockThreshold = lowStockThreshold
	p.IsArchived = isArchived
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes a product by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List trims the search term and validates the sort key before delegating
// to the repository. Invalid sort columns (incl. injection attempts) map
// to ErrValidation.
func (s *Service) List(ctx context.Context, q ListQuery) ([]*Product, int64, error) {
	q.Q = strings.TrimSpace(q.Q)
	if err := validateSort(q.Sort); err != nil {
		return nil, 0, err
	}
	return s.repo.List(ctx, q)
}

// validateSort rejects any sort key outside the whitelist.
func validateSort(raw string) error {
	col := strings.TrimPrefix(raw, "-")
	switch col {
	case "", "name", "price", "created_at", "sku":
		return nil
	default:
		return sharederr.ErrValidation
	}
}

func trimDescription(d *string) *string {
	if d == nil {
		return nil
	}
	t := strings.TrimSpace(*d)
	if t == "" {
		return nil
	}
	return &t
}
