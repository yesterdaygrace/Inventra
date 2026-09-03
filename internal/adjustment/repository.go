// GORM-backed repository for adjustment requests.
package adjustment

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory/internal/shared/dbutil"
	sharederr "inventory/internal/shared/errors"
)

// GORMRepository persists adjustments using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// AdjustmentView is an adjustment joined with product identity for lists.
type AdjustmentView struct {
	ID              uuid.UUID  `gorm:"column:id"`
	ProductID       uuid.UUID  `gorm:"column:product_id"`
	ProductSKU      string     `gorm:"column:product_sku"`
	ProductName     string     `gorm:"column:product_name"`
	WarehouseID     uuid.UUID  `gorm:"column:warehouse_id"`
	SystemQuantity  int        `gorm:"column:system_quantity"`
	CountedQuantity int        `gorm:"column:counted_quantity"`
	Reason          string     `gorm:"column:reason"`
	Note            *string    `gorm:"column:note"`
	Status          string     `gorm:"column:status"`
	RequestedBy     *uuid.UUID `gorm:"column:requested_by"`
	ReviewedBy      *uuid.UUID `gorm:"column:reviewed_by"`
	AppliedValue    *float64   `gorm:"column:applied_value"`
	CreatedAt       string     `gorm:"column:created_at"`
}

// ListQuery filters and paginates adjustments.
type ListQuery struct {
	Status  string
	Page    int
	PerPage int
}

// LastKnownCost returns the most recent RECEIVE unit cost for the pair,
// falling back to the product's list price. Mirrors dashboard valuation.
func (r *GORMRepository) LastKnownCost(ctx context.Context, productID, warehouseID uuid.UUID) (float64, error) {
	var cost struct {
		Value *float64
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT l.unit_cost AS value
		FROM inventory_ledger l
		WHERE l.product_id = ? AND l.warehouse_id = ?
		  AND l.transaction_type = 'RECEIVE' AND l.unit_cost IS NOT NULL
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT 1`, productID, warehouseID).Scan(&cost).Error
	if err != nil {
		return 0, err
	}
	if cost.Value != nil {
		return *cost.Value, nil
	}
	var price struct {
		Price float64
	}
	err = r.db.WithContext(ctx).Table("products").
		Select("price").Where("id = ?", productID).Take(&price).Error
	if err != nil {
		return 0, err
	}
	return price.Price, nil
}

// SystemQuantity reads current on-hand quantity for the pair (0 when absent).
func (r *GORMRepository) SystemQuantity(ctx context.Context, productID, warehouseID uuid.UUID) (int, error) {
	var row struct {
		Quantity int
	}
	err := r.db.WithContext(ctx).Table("inventory").
		Select("COALESCE(quantity, 0) AS quantity").
		Where("product_id = ? AND warehouse_id = ?", productID, warehouseID).
		Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.Quantity, nil
}

// Threshold reads the approval threshold from system_settings.
func (r *GORMRepository) Threshold(ctx context.Context) (float64, error) {
	var row SystemSetting
	err := r.db.WithContext(ctx).Where("key = ?", SettingApprovalThreshold).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return 500, nil // sane default if settings were never seeded
	}
	if err != nil {
		return 0, err
	}
	var v float64
	if _, err := fmt.Sscanf(row.Value, "%f", &v); err != nil {
		return 500, nil
	}
	return v, nil
}

// Create inserts a new adjustment request.
func (r *GORMRepository) Create(ctx context.Context, a *Adjustment) error {
	return r.db.WithContext(ctx).Create(a).Error
}

// GetByID locks and returns one adjustment.
func (r *GORMRepository) GetByID(ctx context.Context, id uuid.UUID) (*Adjustment, error) {
	var a Adjustment
	err := r.db.WithContext(ctx).First(&a, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, sharederr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// Save persists status transitions and review metadata.
func (r *GORMRepository) Save(ctx context.Context, a *Adjustment) error {
	return r.db.WithContext(ctx).Save(a).Error
}

// List returns filtered, paginated adjustments joined with product identity.
func (r *GORMRepository) List(ctx context.Context, q ListQuery) ([]*AdjustmentView, int64, error) {
	if q.Status != "" && q.Status != StatusPending && q.Status != StatusApproved && q.Status != StatusRejected {
		return nil, 0, sharederr.ErrValidation
	}

	db := r.db.WithContext(ctx).Table("inventory_adjustments AS a").
		Select(strings.Join([]string{
			"a.id", "a.product_id", "p.sku AS product_sku", "p.name AS product_name",
			"a.warehouse_id", "a.system_quantity", "a.counted_quantity",
			"a.reason", "a.note", "a.status", "a.requested_by", "a.reviewed_by",
			"a.applied_value", "to_char(a.created_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS created_at",
		}, ", ")).
		Joins("JOIN products p ON p.id = a.product_id")

	if q.Status != "" {
		db = db.Where("a.status = ?", q.Status)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	p, per := dbutil.NormalizePage(q.Page, q.PerPage)
	var views []*AdjustmentView
	if err := db.Order("a.created_at DESC, a.id DESC").
		Offset((p - 1) * per).
		Limit(per).
		Scan(&views).Error; err != nil {
		return nil, 0, err
	}
	return views, total, nil
}
