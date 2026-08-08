// Service logic for stock movements. Consumes a Repository interface and
// returns sentinel-wrapped errors.
package inventory

import (
	"context"

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

// InventoryView is a product joined with its current stock quantity. Every
// product is surfaced even when no inventory row exists yet (quantity 0).
type InventoryView struct {
	ProductID   uuid.UUID `gorm:"column:product_id"`
	ProductSKU  string    `gorm:"column:product_sku"`
	ProductName string    `gorm:"column:product_name"`
	Quantity    int       `gorm:"column:quantity"`
	UpdatedAt   string    `gorm:"column:updated_at"`
}

// TransactionView is a stock movement joined with its product identity.
type TransactionView struct {
	ID          uuid.UUID  `gorm:"column:id"`
	ProductID   uuid.UUID  `gorm:"column:product_id"`
	ProductSKU  string     `gorm:"column:product_sku"`
	ProductName string     `gorm:"column:product_name"`
	Type        string     `gorm:"column:type"`
	Quantity    int        `gorm:"column:quantity"`
	UnitCost    *float64   `gorm:"column:unit_cost"`
	Note        *string    `gorm:"column:note"`
	UserID      *uuid.UUID `gorm:"column:user_id"`
	CreatedAt   string     `gorm:"column:created_at"`
}

// ListQuery filters and paginates the joined inventory view.
type ListQuery struct {
	ProductID uuid.UUID
	Search    string // product name or SKU
	LowStock  bool
	Page      int
	PerPage   int
}

// TransactionQuery filters and paginates the transaction history.
type TransactionQuery struct {
	ProductID uuid.UUID
	Type      string // "", "IN" or "OUT"
	Page      int
	PerPage   int
}

// Repository abstracts persistence for the inventory service.
type Repository interface {
	StockIn(ctx context.Context, m Movement) (*Inventory, error)
	StockOut(ctx context.Context, m Movement) (*Inventory, error)
	List(ctx context.Context, q ListQuery) ([]*InventoryView, int64, error)
	Transactions(ctx context.Context, q TransactionQuery) ([]*TransactionView, int64, error)
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
func (s *Service) StockIn(ctx context.Context, m Movement) (*Inventory, error) {
	if err := validateMovement(m); err != nil {
		return nil, err
	}
	return s.repo.StockIn(ctx, m)
}

// StockOut decreases stock for a product and records the movement atomically.
func (s *Service) StockOut(ctx context.Context, m Movement) (*Inventory, error) {
	if err := validateMovement(m); err != nil {
		return nil, err
	}
	return s.repo.StockOut(ctx, m)
}

// List returns a filtered, paginated joined inventory view plus the total.
func (s *Service) List(ctx context.Context, q ListQuery) ([]*InventoryView, int64, error) {
	return s.repo.List(ctx, q)
}

// Transactions returns a filtered, paginated movement history plus the total.
func (s *Service) Transactions(ctx context.Context, q TransactionQuery) ([]*TransactionView, int64, error) {
	return s.repo.Transactions(ctx, q)
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
