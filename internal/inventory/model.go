// Package inventory implements stock tracking with current quantities and transaction history.
package inventory

import (
	"time"

	"github.com/google/uuid"

	"inventory/internal/product"
)

// Inventory represents current stock quantity for a product (1:1 relationship).
type Inventory struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ProductID uuid.UUID       `gorm:"type:uuid;unique;not null"`
	Product   product.Product `gorm:"foreignKey:ProductID"`
	Quantity  int             `gorm:"not null;default:0;check:quantity >= 0"`
	UpdatedAt time.Time       `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (Inventory) TableName() string {
	return "inventory"
}

// InventoryTransaction represents a stock movement event (IN or OUT).
type InventoryTransaction struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ProductID uuid.UUID       `gorm:"type:uuid;not null"`
	Product   product.Product `gorm:"foreignKey:ProductID"`
	Type      string          `gorm:"type:text;not null;check:type IN ('IN', 'OUT')"`
	Quantity  int             `gorm:"not null;check:quantity > 0"`
	UnitCost  *float64        `gorm:"type:numeric(12,2)"`
	Note      *string         `gorm:"type:text"`
	UserID    *uuid.UUID      `gorm:"type:uuid"`
	CreatedAt time.Time       `gorm:"autoCreateTime"`
}

// TableName overrides the default table name.
func (InventoryTransaction) TableName() string {
	return "inventory_transactions"
}
