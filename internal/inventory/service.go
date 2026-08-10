// Service logic for stock movements. Consumes a Repository interface and
// returns sentinel-wrapped errors.
package inventory

import (
	"context"

	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
)

// Movement describes a stock change for a single product in a specific warehouse.
type Movement struct {
	ProductID   uuid.UUID
	Type        string // "IN" or "OUT"
	Quantity    int
	UnitCost    *float64
	Note        *string
	UserID      *uuid.UUID
	WarehouseID *uuid.UUID // nil → resolved to the seeded DEFAULT warehouse
}

// Transfer describes a warehouse-to-warehouse stock movement for one product.
type Transfer struct {
	ProductID       uuid.UUID
	FromWarehouseID uuid.UUID
	ToWarehouseID   uuid.UUID
	Quantity        int
	Note            *string
	UserID          *uuid.UUID
}

// InventoryView is a product joined with its aggregated (or per-warehouse)
// current stock quantity. Every product is surfaced even when no inventory
// row exists yet (quantity 0). When WarehouseID is filtered, the view is
// scoped to that single warehouse; otherwise it aggregates across all locations.
type InventoryView struct {
	ProductID   uuid.UUID  `gorm:"column:product_id"`
	ProductSKU  string     `gorm:"column:product_sku"`
	ProductName string     `gorm:"column:product_name"`
	Quantity    int        `gorm:"column:quantity"`
	WarehouseID *uuid.UUID `gorm:"column:warehouse_id"`
	UpdatedAt   string     `gorm:"column:updated_at"`
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
	WarehouseID *uuid.UUID `gorm:"column:warehouse_id"`
	TransferID  *uuid.UUID `gorm:"column:transfer_id"`
	CreatedAt   string     `gorm:"column:created_at"`
}

// ListQuery filters and paginates the inventory view.
type ListQuery struct {
	ProductID   uuid.UUID
	Search      string
	LowStock    bool
	WarehouseID *uuid.UUID
	Page        int
	PerPage     int
}

// TransactionQuery filters and paginates the transaction history.
type TransactionQuery struct {
	ProductID   uuid.UUID
	Type        string
	WarehouseID *uuid.UUID
	Page        int
	PerPage     int
}

// Repository abstracts persistence for the inventory service.
type Repository interface {
	StockIn(ctx context.Context, m Movement) (*Inventory, error)
	StockOut(ctx context.Context, m Movement) (*Inventory, error)
	Transfer(ctx context.Context, t Transfer) (*Inventory, error)
	List(ctx context.Context, q ListQuery) ([]*InventoryView, int64, error)
	Transactions(ctx context.Context, q TransactionQuery) ([]*TransactionView, int64, error)
	DefaultWarehouse(ctx context.Context) (uuid.UUID, error)
}

// Service orchestrates stock movements.
type Service struct {
	repo Repository
}

// NewService wires a repository into the service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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

func validateTransfer(t Transfer) error {
	if t.ProductID == uuid.Nil {
		return sharederr.ErrValidation
	}
	if t.FromWarehouseID == uuid.Nil || t.ToWarehouseID == uuid.Nil {
		return sharederr.ErrValidation
	}
	if t.FromWarehouseID == t.ToWarehouseID {
		return sharederr.ErrValidation
	}
	if t.Quantity <= 0 {
		return sharederr.ErrValidation
	}
	return nil
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

// Transfer moves stock between two warehouses for the same product in a
// single atomic transaction.
func (s *Service) Transfer(ctx context.Context, t Transfer) (*Inventory, error) {
	if err := validateTransfer(t); err != nil {
		return nil, err
	}
	return s.repo.Transfer(ctx, t)
}

// List returns a filtered, paginated joined inventory view plus the total.
func (s *Service) List(ctx context.Context, q ListQuery) ([]*InventoryView, int64, error) {
	return s.repo.List(ctx, q)
}

// Transactions returns a filtered, paginated movement history plus the total.
func (s *Service) Transactions(ctx context.Context, q TransactionQuery) ([]*TransactionView, int64, error) {
	return s.repo.Transactions(ctx, q)
}
