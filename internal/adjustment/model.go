// Package adjustment implements stock correction requests with an approval
// workflow (PRD §23). Stock is never edited directly: a request snapshots
// the system quantity, and approval applies the counted quantity through
// the inventory module's ApplyCorrection, writing an ADJUSTMENT ledger row.
package adjustment

import (
	"time"

	"github.com/google/uuid"
)

// Adjustment reasons (PRD §23 enum).
const (
	ReasonCountVariance    = "COUNT_VARIANCE"
	ReasonDamage           = "DAMAGE"
	ReasonTheft            = "THEFT"
	ReasonExpiredStock     = "EXPIRED_STOCK"
	ReasonSupplierShortage = "SUPPLIER_SHORTAGE"
	ReasonSystemCorrection = "SYSTEM_CORRECTION"
	ReasonOther            = "OTHER"
)

var reasonSet = map[string]bool{
	ReasonCountVariance: true, ReasonDamage: true, ReasonTheft: true,
	ReasonExpiredStock: true, ReasonSupplierShortage: true,
	ReasonSystemCorrection: true, ReasonOther: true,
}

// Adjustment statuses.
const (
	StatusPending  = "PENDING"
	StatusApproved = "APPROVED"
	StatusRejected = "REJECTED"
)

// Adjustment is one requested stock correction.
type Adjustment struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ProductID       uuid.UUID  `gorm:"type:uuid;not null"`
	WarehouseID     uuid.UUID  `gorm:"type:uuid;not null"`
	SystemQuantity  int        `gorm:"not null;check:system_quantity >= 0"`
	CountedQuantity int        `gorm:"not null;check:counted_quantity >= 0"`
	Reason          string     `gorm:"type:text;not null;check:reason IN ('COUNT_VARIANCE','DAMAGE','THEFT','EXPIRED_STOCK','SUPPLIER_SHORTAGE','SYSTEM_CORRECTION','OTHER')"`
	Note            *string    `gorm:"type:text"`
	Status          string     `gorm:"type:text;not null;default:PENDING;check:status IN ('PENDING','APPROVED','REJECTED')"`
	RequestedBy     *uuid.UUID `gorm:"type:uuid"`
	ReviewedBy      *uuid.UUID `gorm:"type:uuid"`
	ReviewedAt      *time.Time `gorm:"type:timestamptz"`
	AppliedValue    *float64   `gorm:"type:numeric(14,2)"`
	CreatedAt       time.Time  `gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (Adjustment) TableName() string { return "inventory_adjustments" }

// SystemSetting is one key/value row of runtime configuration.
type SystemSetting struct {
	Key       string    `gorm:"primaryKey"`
	Value     string    `gorm:"not null"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName overrides the default table name.
func (SystemSetting) TableName() string { return "system_settings" }

// Setting keys.
const (
	SettingApprovalThreshold = "adjustment_approval_threshold"
)
