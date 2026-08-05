// GORM-backed implementation of the category Repository interface.
package category

import (
	"strings"

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
func (r *GORMRepository) Create(c *Category) error {
	if err := r.db.Create(c).Error; err != nil {
		if isUniqueViolation(err) {
			return sharederr.ErrConflict
		}
		return err
	}
	return nil
}

// Get returns a category by ID.
func (r *GORMRepository) Get(id uuid.UUID) (*Category, error) {
	var c Category
	if err := r.db.Where("id = ?", id).First(&c).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// Update persists category changes, translating unique-name violations.
func (r *GORMRepository) Update(c *Category) error {
	if err := r.db.Save(c).Error; err != nil {
		if isUniqueViolation(err) {
			return sharederr.ErrConflict
		}
		return err
	}
	return nil
}

// Delete removes a category row.
func (r *GORMRepository) Delete(id uuid.UUID) error {
	res := r.db.Delete(&Category{}, "id = ?", id)
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
func (r *GORMRepository) List(q ListQuery) ([]*Category, int64, error) {
	db := r.db.Model(&Category{})

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
	switch {
	case q.Sort == "-name":
		order = "name DESC"
	case q.Sort == "created_at":
		order = "created_at ASC"
	case q.Sort == "-created_at":
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
func (r *GORMRepository) CountProductsFor(id uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Table("products").
		Where("category_id = ?", id).
		Count(&count).Error
	return count, err
}

// isUniqueViolation detects a PostgreSQL unique-constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}
