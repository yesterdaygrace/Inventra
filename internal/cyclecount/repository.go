// GORM-backed repository for cycle counting.
package cyclecount

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	sharederr "inventory/internal/shared/errors"
)

// GORMRepository persists cycle count plans and items.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// PlanView is a plan with its counting progress.
type PlanView struct {
	ID            uuid.UUID `gorm:"column:id"`
	WarehouseID   uuid.UUID `gorm:"column:warehouse_id"`
	Name          string    `gorm:"column:name"`
	Status        string    `gorm:"column:status"`
	TotalItems    int64     `gorm:"column:total_items"`
	CountedItems  int64     `gorm:"column:counted_items"`
	VarianceItems int64     `gorm:"column:variance_items"`
	CreatedAt     string    `gorm:"column:created_at"`
}

// ItemView is a counted/uncounted item joined with product identity.
type ItemView struct {
	ID              uuid.UUID  `gorm:"column:id"`
	PlanID          uuid.UUID  `gorm:"column:plan_id"`
	ProductID       uuid.UUID  `gorm:"column:product_id"`
	ProductSKU      string     `gorm:"column:product_sku"`
	ProductName     string     `gorm:"column:product_name"`
	SystemQuantity  int        `gorm:"column:system_quantity"`
	CountedQuantity *int       `gorm:"column:counted_quantity"`
	Status          string     `gorm:"column:status"`
	AdjustmentID    *uuid.UUID `gorm:"column:adjustment_id"`
	CountedAt       *string    `gorm:"column:counted_at"`
}

// CreatePlan inserts the plan and its items in one transaction.
func (r *GORMRepository) CreatePlan(ctx context.Context, plan *Plan, items []Item) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(plan).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].PlanID = plan.ID
		}
		return tx.Create(&items).Error
	})
}

// GetPlan returns one plan or ErrNotFound.
func (r *GORMRepository) GetPlan(ctx context.Context, id uuid.UUID) (*Plan, error) {
	var p Plan
	err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, sharederr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SavePlan persists plan status transitions.
func (r *GORMRepository) SavePlan(ctx context.Context, p *Plan) error {
	return r.db.WithContext(ctx).Save(p).Error
}

// GetItem locks and returns one item.
func (r *GORMRepository) GetItem(ctx context.Context, id uuid.UUID) (*Item, error) {
	var it Item
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&it, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, sharederr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// SaveItem persists a count result.
func (r *GORMRepository) SaveItem(ctx context.Context, it *Item) error {
	return r.db.WithContext(ctx).Save(it).Error
}

// CountPendingItems returns how many items in the plan are still uncounted.
func (r *GORMRepository) CountPendingItems(ctx context.Context, planID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&Item{}).
		Where("plan_id = ? AND counted_quantity IS NULL", planID).
		Count(&n).Error
	return n, err
}

// ListPlans returns plans with progress aggregates, newest first.
func (r *GORMRepository) ListPlans(ctx context.Context) ([]*PlanView, int64, error) {
	db := r.db.WithContext(ctx).Table("cycle_count_plans AS p").
		Select(strings_join([]string{
			"p.id", "p.warehouse_id", "p.name", "p.status",
			"COUNT(i.id) AS total_items",
			"COUNT(i.counted_quantity) AS counted_items",
			"COALESCE(SUM(CASE WHEN i.counted_quantity IS NOT NULL AND i.counted_quantity <> i.system_quantity THEN 1 ELSE 0 END), 0) AS variance_items",
			"to_char(p.created_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS created_at",
		}, ", ")).
		Joins("LEFT JOIN cycle_count_items i ON i.plan_id = p.id").
		Group("p.id")

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var views []*PlanView
	if err := db.Order("p.created_at DESC").Scan(&views).Error; err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// PlanItems lists a plan's items joined with product identity.
func (r *GORMRepository) PlanItems(ctx context.Context, planID uuid.UUID) ([]*ItemView, error) {
	var views []*ItemView
	err := r.db.WithContext(ctx).Table("cycle_count_items AS i").
		Select(strings_join([]string{
			"i.id", "i.plan_id", "i.product_id", "pr.sku AS product_sku",
			"pr.name AS product_name", "i.system_quantity", "i.counted_quantity",
			"CASE WHEN i.counted_quantity IS NULL THEN 'PENDING' ELSE 'COUNTED' END AS status",
			"i.adjustment_id",
			"to_char(i.counted_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS counted_at",
		}, ", ")).
		Joins("JOIN products pr ON pr.id = i.product_id").
		Where("i.plan_id = ?", planID).
		Order("pr.name ASC").
		Scan(&views).Error
	return views, err
}

// strings_join avoids importing strings just for Join in this file.
func strings_join(parts []string, sep string) string {
	out := ""
	for i, s := range parts {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
