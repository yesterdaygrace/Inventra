// Service logic for stock movements. Consumes a Repository interface and
// returns sentinel-wrapped errors.
package inventory

import (
	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
)

// Movement describes a stock change for a single product.
type Movement struct {
	ProductID uuid.UUID
	Type      string // "IN" or "OUT"
	Quantity  int
	UnitCost  *float64
	Note      *string
	UserID    *uuid.UUID
}

// Repository abstracts persistence for the inventory service.
type Repository interface {
	StockIn(m Movement) (*Inventory, error)
	StockOut(m Movement) (*Inventory, error)
}

// Service orchestrates stock movements.
type Service struct {
	repo Repository
}

// NewService wires a repository into the service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// StockIn increases stock for a product and records the movement atomically.
func (s *Service) StockIn(m Movement) (*Inventory, error) {
	if err := validateMovement(m); err != nil {
		return nil, err
	}
	return s.repo.StockIn(m)
}

// StockOut decreases stock for a product and records the movement atomically.
func (s *Service) StockOut(m Movement) (*Inventory, error) {
	if err := validateMovement(m); err != nil {
		return nil, err
	}
	return s.repo.StockOut(m)
}

func validateMovement(m Movement) error {
	if m.ProductID == uuid.Nil {
		return sharederr.ErrValidation
	}
	if m.Type != "IN" && m.Type != "OUT" {
		return sharederr.ErrValidation
	}
	if m.Quantity <= 0 {
		return sharederr.ErrValidation
	}
	return nil
}
