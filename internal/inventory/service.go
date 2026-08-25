// Service logic for stock movements. Consumes a Repository interface and
// returns sentinel-wrapped errors.
package inventory

import (
	"context"
	"time"

	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
)

// Movement describes a stock change for a single product in a specific warehouse.
type Movement struct {
	ProductID     uuid.UUID
	Type          string // "RECEIVE" or "ISSUE"
	Quantity      int
	UnitCost      *float64
	Note          *string
	UserID        *uuid.UUID
	WarehouseID   *uuid.UUID // nil → resolved to the seeded DEFAULT warehouse
	ReferenceType *string
	ReferenceID   *string
	Reason        *string
}

// Transfer describes a warehouse-to-warehouse stock movement for one product.
type Transfer struct {
	ProductID       uuid.UUID
	FromWarehouseID uuid.UUID
	ToWarehouseID   uuid.UUID
	Quantity        int
	Note            *string
	UserID          *uuid.UUID
	ReferenceType   *string
	ReferenceID     *string
	Reason          *string
}

// InventoryView is a product joined with its aggregated (or per-warehouse)
// current stock quantity. Every product is surfaced even when no inventory
// row exists yet (quantity 0). When WarehouseID is filtered, the view is
// scoped to that single warehouse; otherwise it aggregates across all locations.
type InventoryView struct {
	ProductID        uuid.UUID  `gorm:"column:product_id"`
	ProductSKU       string     `gorm:"column:product_sku"`
	ProductName      string     `gorm:"column:product_name"`
	Quantity         int        `gorm:"column:quantity"`
	ReservedQuantity int        `gorm:"column:reserved_quantity"`
	Version          int        `gorm:"column:version"`
	WarehouseID      *uuid.UUID `gorm:"column:warehouse_id"`
	UpdatedAt        string     `gorm:"column:updated_at"`
}

// LedgerView is one ledger line joined with its product identity, carrying
// the running balance for its (product, warehouse) pair computed on read.
type LedgerView struct {
	ID              uuid.UUID  `gorm:"column:id"`
	ProductID       uuid.UUID  `gorm:"column:product_id"`
	ProductSKU      string     `gorm:"column:product_sku"`
	ProductName     string     `gorm:"column:product_name"`
	TransactionType string     `gorm:"column:transaction_type"`
	Direction       string     `gorm:"column:direction"`
	Quantity        int        `gorm:"column:quantity"`
	Balance         int        `gorm:"column:balance"`
	UnitCost        *float64   `gorm:"column:unit_cost"`
	TotalCost       *float64   `gorm:"column:total_cost"`
	Note            *string    `gorm:"column:note"`
	UserID          *uuid.UUID `gorm:"column:performed_by"`
	WarehouseID     uuid.UUID  `gorm:"column:warehouse_id"`
	TransferID      *uuid.UUID `gorm:"column:transfer_id"`
	ReferenceType   *string    `gorm:"column:reference_type"`
	ReferenceID     *string    `gorm:"column:reference_id"`
	Reason          *string    `gorm:"column:reason"`
	CreatedAt       string     `gorm:"column:created_at"`
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

// LedgerQuery filters and paginates the ledger history.
type LedgerQuery struct {
	ProductID   uuid.UUID
	Type        string
	WarehouseID *uuid.UUID
	Page        int
	PerPage     int
}

// Repository abstracts persistence for the inventory service.
type Repository interface {
	Receive(ctx context.Context, m Movement) (*Inventory, error)
	Issue(ctx context.Context, m Movement) (*Inventory, error)
	Transfer(ctx context.Context, t Transfer) (*Inventory, error)
	List(ctx context.Context, q ListQuery) ([]*InventoryView, int64, error)
	Ledger(ctx context.Context, q LedgerQuery) ([]*LedgerView, int64, error)
	CreateReservation(ctx context.Context, rsv Reservation) (*Reservation, error)
	ReleaseReservation(ctx context.Context, id uuid.UUID) (*Reservation, error)
	ConsumeReservation(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*Reservation, *Inventory, error)
	Reservations(ctx context.Context, q ReservationQuery) ([]*ReservationView, int64, error)
	ApplyCorrection(ctx context.Context, productID, warehouseID uuid.UUID, targetQuantity int, referenceType, referenceID, reason string, userID *uuid.UUID) (*Inventory, error)
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
	if m.Type != LedgerReceive && m.Type != LedgerIssue {
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

// Receive increases stock for a product and appends a RECEIVE ledger entry
// in the same transaction.
func (s *Service) Receive(ctx context.Context, m Movement) (*Inventory, error) {
	if err := validateMovement(m); err != nil {
		return nil, err
	}
	return s.repo.Receive(ctx, m)
}

// Issue decreases stock for a product and appends an ISSUE ledger entry
// in the same transaction.
func (s *Service) Issue(ctx context.Context, m Movement) (*Inventory, error) {
	if err := validateMovement(m); err != nil {
		return nil, err
	}
	return s.repo.Issue(ctx, m)
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

// Ledger returns a filtered, paginated ledger history plus the total.
func (s *Service) Ledger(ctx context.Context, q LedgerQuery) ([]*LedgerView, int64, error) {
	return s.repo.Ledger(ctx, q)
}

// ReservationInput is a validated request to hold stock for a reference.
type ReservationInput struct {
	ProductID     uuid.UUID
	WarehouseID   uuid.UUID
	Quantity      int
	ReferenceType string
	ReferenceID   string
	ExpiresAt     *time.Time
}

// CreateReservation holds available stock for a reference (PRD §21).
func (s *Service) CreateReservation(ctx context.Context, in ReservationInput) (*Reservation, error) {
	if in.ProductID == uuid.Nil || in.WarehouseID == uuid.Nil {
		return nil, sharederr.ErrValidation
	}
	if in.Quantity <= 0 {
		return nil, sharederr.ErrValidation
	}
	if in.ReferenceType == "" || in.ReferenceID == "" {
		return nil, sharederr.ErrValidation
	}
	if in.ExpiresAt != nil && in.ExpiresAt.Before(time.Now()) {
		return nil, sharederr.ErrValidation
	}
	return s.repo.CreateReservation(ctx, Reservation{
		ProductID:     in.ProductID,
		WarehouseID:   in.WarehouseID,
		Quantity:      in.Quantity,
		ReferenceType: in.ReferenceType,
		ReferenceID:   in.ReferenceID,
		Status:        ReservationActive,
		ExpiresAt:     in.ExpiresAt,
	})
}

// ReleaseReservation returns a reservation's quantity to available stock.
func (s *Service) ReleaseReservation(ctx context.Context, id uuid.UUID) (*Reservation, error) {
	if id == uuid.Nil {
		return nil, sharederr.ErrValidation
	}
	return s.repo.ReleaseReservation(ctx, id)
}

// ConsumeReservation converts a reservation into an ISSUE ledger entry.
func (s *Service) ConsumeReservation(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*Reservation, *Inventory, error) {
	if id == uuid.Nil {
		return nil, nil, sharederr.ErrValidation
	}
	return s.repo.ConsumeReservation(ctx, id, userID)
}

// Reservations lists reservations (ACTIVE by default).
func (s *Service) Reservations(ctx context.Context, q ReservationQuery) ([]*ReservationView, int64, error) {
	return s.repo.Reservations(ctx, q)
}

// ApplyCorrection sets stock to an exact quantity with an ADJUSTMENT ledger
// entry. Exposed for the adjustment module's approval flow.
func (s *Service) ApplyCorrection(ctx context.Context, productID, warehouseID uuid.UUID, targetQuantity int, referenceType, referenceID, reason string, userID *uuid.UUID) (*Inventory, error) {
	if productID == uuid.Nil || warehouseID == uuid.Nil || targetQuantity < 0 {
		return nil, sharederr.ErrValidation
	}
	return s.repo.ApplyCorrection(ctx, productID, warehouseID, targetQuantity, referenceType, referenceID, reason, userID)
}
