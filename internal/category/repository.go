// GORM-backed implementation of the category Repository interface.
package category

import (
	"context"
	"strings"

	"inventory/internal/shared/dbutil"

	"github.com/google/uuid"
	"gorm.io/gorm"

	sharederr "inventory/internal/shared/errors"
)

// GORMRepository persists categories using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// Create persists a new category, translating unique-name violations.
func (r *GORMRepository) Create(ctx context.Context, c *Category) error {
	if err := r.db.WithContext(ctx).Create(c).Error; err != nil {
		if dbutil.IsUniqueViolation(err) {
			return sharederr.ErrConflict
		}
		return err
	}
	return nil
}

// Get returns a category by ID.
func (r *GORMRepository) Get(ctx context.Context, id uuid.UUID) (*Category, error) {
	var c Category
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// Update persists category changes, translating unique-name violations.
func (r *GORMRepository) Update(ctx context.Context, c *Category) error {
	if err := r.db.WithContext(ctx).Save(c).Error; err != nil {
		if dbutil.IsUniqueViolation(err) {
			return sharederr.ErrConflict
		}
		return err
	}
	return nil
}

// Delete removes a category row.
func (r *GORMRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&Category{}).
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

// List returns a page of categories matching the search, sorted, plus the
// total match count before paging.
func (r *GORMRepository) List(ctx context.Context, q ListQuery) ([]*Category, int64, error) {
	db := r.db.WithContext(ctx).Model(&Category{})

	if q.Search != "" {
		db = db.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(q.Search)+"%")
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
	}

	var cats []*Category
	if err := db.Order(order).
		Offset((q.Page - 1) * q.PerPage).
		Limit(q.PerPage).
		Find(&cats).Error; err != nil {
		return nil, 0, err
	}
	return cats, total, nil
}

// CountProductsFor counts products referencing the given category.
// It queries the physical products table directly (product imports
// category, so importing the model here would create an import cycle).
func (r *GORMRepository) CountProductsFor(ctx context.Context, id uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("products").
		Where("category_id = ? AND is_archived = ?", id, false).
		Count(&count).Error
	return count, err
}
