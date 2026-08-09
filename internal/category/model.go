// Package category implements the product category domain.
package category

import (
	"time"

	"github.com/google/uuid"
)

// Category represents a product category for organization and reporting.
type Category struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name         string    `gorm:"type:text;unique;not null"`
	Description  *string   `gorm:"type:text"`
	IsActive     bool      `gorm:"not null;default:true"`
	ProductCount int64     `gorm:"->;-:migration"` // computed by List; never stored
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (Category) TableName() string {
	return "categories"
}
