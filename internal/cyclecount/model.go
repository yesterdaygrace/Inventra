// Package cyclecount implements warehouse-scoped cycle counting (PRD §24).
// A plan snapshots system quantities for an explicit SKU list; counting an
// item with a variance files an adjustment request through the §23
// approval workflow, so stock only moves via approved corrections.
package cyclecount

import (
	"time"

	"github.com/google/uuid"
)

// Plan statuses.
const (
	PlanOpen      = "OPEN"
	PlanCompleted = "COMPLETED"
)

// Plan is one warehouse-scoped counting session.
type Plan struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WarehouseID uuid.UUID  `gorm:"type:uuid;not null"`
	Name        string     `gorm:"type:text;not null"`
	Status      string     `gorm:"type:text;not null;default:OPEN;check:status IN ('OPEN','COMPLETED')"`
	CreatedBy   *uuid.UUID `gorm:"type:uuid"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (Plan) TableName() string { return "cycle_count_plans" }

// Item statuses derived from data: counted_quantity IS NULL → PENDING.
const (
	ItemPending = "PENDING"
	ItemCounted = "COUNTED"
)

// Item is one SKU inside a plan, snapshotted at creation time.
type Item struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PlanID          uuid.UUID  `gorm:"type:uuid;not null"`
	ProductID       uuid.UUID  `gorm:"type:uuid;not null"`
	SystemQuantity  int        `gorm:"not null;check:system_quantity >= 0"`
	CountedQuantity *int       `gorm:"check:counted_quantity >= 0"`
	CountedBy       *uuid.UUID `gorm:"type:uuid"`
	CountedAt       *time.Time `gorm:"type:timestamptz"`
	AdjustmentID    *uuid.UUID `gorm:"type:uuid"`
	CreatedAt       time.Time  `gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (Item) TableName() string { return "cycle_count_items" }
