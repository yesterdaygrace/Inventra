// Package product implements the product domain with SKU, pricing, and archival.
package product

import (
	"time"

	"github.com/google/uuid"

	"inventory/internal/category"
)

// Product represents an inventory item with SKU, price, and category.
type Product struct {
	ID                uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name              string            `gorm:"type:text;not null"`
	SKU               string            `gorm:"type:text;unique;not null"`
	Description       *string           `gorm:"type:text"`
	Price             float64           `gorm:"type:numeric(12,2);not null"`
	CategoryID        uuid.UUID         `gorm:"type:uuid;not null"`
	Category          category.Category `gorm:"foreignKey:CategoryID"`
	LowStockThreshold int               `gorm:"not null;default:10;check:low_stock_threshold >= 0"`
	IsArchived        bool              `gorm:"not null;default:false"`
	CreatedAt         time.Time         `gorm:"autoCreateTime"`
	UpdatedAt         time.Time         `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (Product) TableName() string {
	return "products"
}
