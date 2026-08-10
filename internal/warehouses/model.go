// Package warehouses implements the warehouse domain for multi-location stock.
package warehouses

import (
	"time"

	"github.com/google/uuid"
)

// Warehouse represents a physical or logical stock location. Inventory rows
// reference it via warehouse_id; the DEFAULT warehouse is seeded so existing
// single-location flows keep working without code changes.
type Warehouse struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Code           string    `gorm:"type:text;unique;not null"`
	Name           string    `gorm:"type:text;not null"`
	Description    *string   `gorm:"type:text"`
	IsActive       bool      `gorm:"not null;default:true"`
	InventoryCount int64     `gorm:"->;-:migration"` // computed by List; never stored
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (Warehouse) TableName() string {
	return "warehouses"
}
