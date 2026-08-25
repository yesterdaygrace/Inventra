// Package inventory implements stock tracking with current quantities and transaction history.
package inventory

import (
	"time"

	"github.com/google/uuid"

	"inventory/internal/product"
)

// Inventory represents current stock quantity for a product within a single
// warehouse. The composite unique key (product_id, warehouse_id) allows the
// same product to be tracked per location; legacy single-location rows are
// backfilled to the seeded DEFAULT warehouse.
type Inventory struct {
	ID               uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ProductID        uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:idx_inventory_product_warehouse"`
	Product          product.Product `gorm:"foreignKey:ProductID"`
	WarehouseID      uuid.UUID       `gorm:"type:uuid;uniqueIndex:idx_inventory_product_warehouse"`
	Quantity         int             `gorm:"not null;default:0;check:quantity >= 0"`
	ReservedQuantity int             `gorm:"not null;default:0;check:reserved_quantity >= 0"`
	Version          int             `gorm:"not null;default:0"`
	UpdatedAt        time.Time       `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (Inventory) TableName() string {
	return "inventory"
}

// Ledger transaction types (PRD §15). Every stock-changing operation
// writes exactly one ledger row per affected (product, warehouse) pair;
// a transfer writes a TRANSFER_OUT/TRANSFER_IN pair sharing one transfer_id.
const (
	LedgerOpeningBalance = "OPENING_BALANCE"
	LedgerReceive        = "RECEIVE"
	LedgerIssue          = "ISSUE"
	LedgerTransferIn     = "TRANSFER_IN"
	LedgerTransferOut    = "TRANSFER_OUT"
	LedgerAdjustment     = "ADJUSTMENT"
	LedgerReturn         = "RETURN"
)

// ledgerTypeSet is every valid transaction_type value.
var ledgerTypeSet = map[string]bool{
	LedgerOpeningBalance: true,
	LedgerReceive:        true,
	LedgerIssue:          true,
	LedgerTransferIn:     true,
	LedgerTransferOut:    true,
	LedgerAdjustment:     true,
	LedgerReturn:         true,
}

// LedgerEntry is one immutable line in the inventory ledger. Rows are
// append-only: corrections happen as new ADJUSTMENT entries, never edits.
type LedgerEntry struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ProductID       uuid.UUID       `gorm:"type:uuid;not null"`
	Product         product.Product `gorm:"foreignKey:ProductID"`
	WarehouseID     uuid.UUID       `gorm:"type:uuid;not null"`
	BatchID         *uuid.UUID      `gorm:"type:uuid"`
	TransactionType string          `gorm:"type:text;not null;check:transaction_type IN ('OPENING_BALANCE','RECEIVE','ISSUE','TRANSFER_IN','TRANSFER_OUT','ADJUSTMENT','RETURN')"`
	Direction       string          `gorm:"type:text;not null;check:direction IN ('IN','OUT')"`
	Quantity        int             `gorm:"not null;check:quantity > 0"`
	UnitCost        *float64        `gorm:"type:numeric(12,2)"`
	TotalCost       *float64        `gorm:"type:numeric(14,2)"`
	ReferenceType   *string         `gorm:"type:text"`
	ReferenceID     *string         `gorm:"type:text"`
	TransferID      *uuid.UUID      `gorm:"type:uuid"`
	Note            *string         `gorm:"type:text"`
	Reason          *string         `gorm:"type:text"`
	PerformedBy     *uuid.UUID      `gorm:"type:uuid"`
	CreatedAt       time.Time       `gorm:"autoCreateTime"`
}

// TableName overrides the default table name.
func (LedgerEntry) TableName() string {
	return "inventory_ledger"
}

// Reservation statuses (PRD §21).
const (
	ReservationActive   = "ACTIVE"
	ReservationReleased = "RELEASED"
	ReservationConsumed = "CONSUMED"
	ReservationExpired  = "EXPIRED"
)

// reservationStatusSet is every valid reservation status value.
var reservationStatusSet = map[string]bool{
	ReservationActive:   true,
	ReservationReleased: true,
	ReservationConsumed: true,
	ReservationExpired:  true,
}

// Reservation holds stock for a reference until it is consumed, released,
// or lazily expired. Available stock is always On Hand − Reserved.
type Reservation struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ProductID     uuid.UUID       `gorm:"type:uuid;not null"`
	Product       product.Product `gorm:"foreignKey:ProductID"`
	WarehouseID   uuid.UUID       `gorm:"type:uuid;not null"`
	Quantity      int             `gorm:"not null;check:quantity > 0"`
	ReferenceType string          `gorm:"type:text;not null"`
	ReferenceID   string          `gorm:"type:text;not null"`
	Status        string          `gorm:"type:text;not null;default:ACTIVE;check:status IN ('ACTIVE','RELEASED','CONSUMED','EXPIRED')"`
	ExpiresAt     *time.Time      `gorm:"type:timestamptz"`
	CreatedAt     time.Time       `gorm:"autoCreateTime"`
	UpdatedAt     time.Time       `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (Reservation) TableName() string {
	return "inventory_reservations"
}
