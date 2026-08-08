// GORM-backed implementation of the product Repository interface.
package product

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory/internal/shared/dbutil"
	sharederr "inventory/internal/shared/errors"
)

// GORMRepository persists products using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// Create persists a new product, translating unique-SKU and category FK
// violations so API clients receive typed errors instead of a generic 500.
func (r *GORMRepository) Create(ctx context.Context, p *Product) error {
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		switch {
		case dbutil.IsUniqueViolation(err):
			return sharederr.ErrConflict
		case dbutil.IsForeignKeyViolation(err):
			return sharederr.ErrNotFound
		default:
			return err
		}
	}
	return nil
}

// Get returns a product by ID with its category.
func (r *GORMRepository) Get(ctx context.Context, id uuid.UUID) (*Product, error) {
	var p Product
	if err := r.db.WithContext(ctx).Preload("Category").Where("id = ?", id).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// Update persists product changes, translating unique-SKU and category FK
// violations into typed errors instead of a generic 500.
func (r *GORMRepository) Update(ctx context.Context, p *Product) error {
	if err := r.db.WithContext(ctx).Save(p).Error; err != nil {
		switch {
		case dbutil.IsUniqueViolation(err):
			return sharederr.ErrConflict
		case dbutil.IsForeignKeyViolation(err):
			return sharederr.ErrNotFound
		default:
			return err
		}
	}
	return nil
}

// Delete soft-archives a product (sets IsArchived=true) rather than removing
// the row, so historical inventory and audit references remain intact. The
// documented contract is a soft archive, not a physical delete.
func (r *GORMRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&Product{}).
		Where("id = ?", id).
		Update("is_archived", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sharederr.ErrNotFound
	}
	return nil
}

// List returns a filtered, sorted, paginated page of products plus the total
// match count. All dynamic WHERE/ORDER/LIMIT/OFFSET values are parameterized
// or whitelisted, so user input cannot be injected.
func (r *GORMRepository) List(ctx context.Context, q ListQuery) ([]*Product, int64, error) {
	db := r.db.WithContext(ctx).Model(&Product{})

	db = applyFilters(db, q)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q.Page, q.PerPage = dbutil.NormalizePage(q.Page, q.PerPage)
	order := orderBy(q.Sort)

	var prods []*Product
	if err := db.Preload("Category").
		Order(order).
		Offset((q.Page - 1) * q.PerPage).
		Limit(q.PerPage).
		Find(&prods).Error; err != nil {
		return nil, 0, err
	}
	return prods, total, nil
}

// SKUExists reports whether any product (other than excludeID) has the SKU.
func (r *GORMRepository) SKUExists(ctx context.Context, sku string, excludeID uuid.UUID) (bool, error) {
	db := r.db.WithContext(ctx).Model(&Product{}).Where("LOWER(sku) = ?", strings.ToLower(strings.TrimSpace(sku)))
	if excludeID != uuid.Nil {
		db = db.Where("id <> ?", excludeID)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// applyFilters builds the dynamic WHERE clause for a ListQuery. Every value
// is bound as a parameter; nothing from input is concatenated.
func applyFilters(db *gorm.DB, q ListQuery) *gorm.DB {
	if q.Q != "" {
		like := "%" + strings.ToLower(q.Q) + "%"
		db = db.Where("(LOWER(name) LIKE ? OR LOWER(sku) LIKE ?)", like, like)
	}
	if q.CategoryID != uuid.Nil {
		db = db.Where("category_id = ?", q.CategoryID)
	}
	if q.MinPrice != nil {
		db = db.Where("price >= ?", *q.MinPrice)
	}
	if q.MaxPrice != nil {
		db = db.Where("price <= ?", *q.MaxPrice)
	}
	if q.IsArchived != nil {
		db = db.Where("is_archived = ?", *q.IsArchived)
	}
	if q.LowStock {
		// LEFT JOIN the physical inventory table (inventory imports product,
		// so referencing it here would create an import cycle). Products with
		// no inventory row count as zero stock.
		db = db.Joins("LEFT JOIN inventory ON inventory.product_id = products.id").
			Where("COALESCE(inventory.quantity, 0) <= products.low_stock_threshold")
	}
	return db
}

// orderBy maps a whitelisted sort key to an ORDER BY fragment.
func orderBy(raw string) string {
	desc := strings.HasPrefix(raw, "-")
	col := strings.TrimPrefix(raw, "-")
	switch col {
	case "price":
		if desc {
			return "price DESC"
		}
		return "price ASC"
	case "created_at":
		if desc {
			return "created_at DESC"
		}
		return "created_at ASC"
	case "sku":
		if desc {
			return "sku DESC"
		}
		return "sku ASC"
	default:
		if desc {
			return "name DESC"
		}
		return "name ASC"
	}
}
