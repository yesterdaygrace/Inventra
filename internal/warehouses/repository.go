// GORM-backed implementation of the warehouses Repository interface.
package warehouses

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory/internal/shared/dbutil"
	sharederr "inventory/internal/shared/errors"
)

// GORMRepository persists warehouses using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// Create persists a new warehouse, translating unique-code violations.
func (r *GORMRepository) Create(ctx context.Context, w *Warehouse) error {
	if err := r.db.WithContext(ctx).Create(w).Error; err != nil {
		if dbutil.IsUniqueViolation(err) {
			return sharederr.ErrConflict
		}
		return err
	}
	return nil
}

// Get returns a warehouse by ID.
func (r *GORMRepository) Get(ctx context.Context, id uuid.UUID) (*Warehouse, error) {
	var w Warehouse
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&w).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &w, nil
}

// GetByCode returns a warehouse by its unique code string.
func (r *GORMRepository) GetByCode(ctx context.Context, code string) (*Warehouse, error) {
	var w Warehouse
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&w).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &w, nil
}

// Update persists warehouse changes, translating unique-code violations.
func (r *GORMRepository) Update(ctx context.Context, w *Warehouse) error {
	if err := r.db.WithContext(ctx).Save(w).Error; err != nil {
		if dbutil.IsUniqueViolation(err) {
			return sharederr.ErrConflict
		}
		return err
	}
	return nil
}

// Delete soft-deactivates a warehouse row (mirrors category soft-delete).
func (r *GORMRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&Warehouse{}).
		Where("id = ?", id).
		Update("is_active", false)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sharederr.ErrNotFound
	}
	return nil
}

// List returns a page of warehouses matching filters, sorted, plus the
// total match count before paging.
func (r *GORMRepository) List(ctx context.Context, q ListQuery) ([]*Warehouse, int64, error) {
	db := r.db.WithContext(ctx).Model(&Warehouse{})

	if q.Search != "" {
		db = db.Where("LOWER(name) LIKE ? OR LOWER(code) LIKE ?",
			"%"+strings.ToLower(q.Search)+"%", "%"+strings.ToLower(q.Search)+"%")
	}
	if q.IsActive != nil {
		db = db.Where("is_active = ?", *q.IsActive)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.PerPage < 1 || q.PerPage > 100 {
		q.PerPage = 20
	}

	order := "name ASC"
	switch q.Sort {
	case "-name":
		order = "name DESC"
	case "created_at":
		order = "created_at ASC"
	case "-created_at":
		order = "created_at DESC"
	case "code":
		order = "code ASC"
	case "-code":
		order = "code DESC"
	}

	var whs []*Warehouse
	if err := db.Order(order).
		Offset((q.Page - 1) * q.PerPage).
		Limit(q.PerPage).
		Find(&whs).Error; err != nil {
		return nil, 0, err
	}
	return whs, total, nil
}

// CountInventoryFor counts inventory rows referencing the given warehouse.
func (r *GORMRepository) CountInventoryFor(ctx context.Context, id uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("inventory").
		Where("warehouse_id = ?", id).
		Count(&count).Error
	return count, err
}

// ListWithInventoryCount is an extended list that enriches warehouses with
// per-warehouse inventory row counts. Used for the API list response.
func (r *GORMRepository) ListWithInventoryCount(ctx context.Context, q ListQuery) ([]*Warehouse, int64, error) {
	// Sub-query that counts inventory rows per warehouse.
	type invCount struct {
		WarehouseID uuid.UUID `gorm:"column:warehouse_id"`
		Count       int64     `gorm:"column:cnt"`
	}

	var counts []invCount
	r.db.WithContext(ctx).Table("inventory").
		Select("warehouse_id, COUNT(*) AS cnt").
		Group("warehouse_id").
		Find(&counts)

	idx := make(map[uuid.UUID]int64, len(counts))
	for _, c := range counts {
		idx[c.WarehouseID] = c.Count
	}

	whs, total, err := r.List(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	for _, w := range whs {
		w.InventoryCount = idx[w.ID]
	}
	return whs, total, nil
}
