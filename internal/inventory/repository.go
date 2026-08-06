// GORM-backed implementation of the inventory Repository interface.
package inventory

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	sharederr "inventory/internal/shared/errors"
)

// GORMRepository persists inventory using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// StockIn records an IN movement and increments the product quantity in a
// single DB transaction. The inventory row is upserted and projects with no
// existing row start from zero, so the first IN creates it.
func (r *GORMRepository) StockIn(m Movement) (*Inventory, error) {
	var result Inventory
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ?", m.ProductID).First(&inv).Error
		switch {
		case err == nil:
			inv.Quantity += m.Quantity
		case err == gorm.ErrRecordNotFound:
			inv = Inventory{ProductID: m.ProductID, Quantity: m.Quantity}
		default:
			return err
		}

		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		if err := tx.Create(&InventoryTransaction{
			ProductID: m.ProductID,
			Type:      "IN",
			Quantity:  m.Quantity,
			UnitCost:  m.UnitCost,
			Note:      m.Note,
			UserID:    m.UserID,
		}).Error; err != nil {
			return err
		}
		result = inv
		return nil
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &result, nil
}

// StockOut records an OUT movement and decrements the product quantity in a
// single transaction. It rejects any draw that would push stock below zero,
// returning ErrConflict and rolling back so no partial history row remains.
func (r *GORMRepository) StockOut(m Movement) (*Inventory, error) {
	var result Inventory
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ?", m.ProductID).First(&inv).Error
		if err == gorm.ErrRecordNotFound {
			return sharederr.ErrConflict
		}
		if err != nil {
			return err
		}

		if inv.Quantity < m.Quantity {
			return sharederr.ErrConflict
		}
		inv.Quantity -= m.Quantity

		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		if err := tx.Create(&InventoryTransaction{
			ProductID: m.ProductID,
			Type:      "OUT",
			Quantity:  m.Quantity,
			UnitCost:  m.UnitCost,
			Note:      m.Note,
			UserID:    m.UserID,
		}).Error; err != nil {
			return err
		}
		result = inv
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns a filtered, sorted, paginated joined inventory view (every
// product left-joined with its stock row) plus the total match count. All
// dynamic values are parameterized, so input cannot be injected.
func (r *GORMRepository) List(q ListQuery) ([]*InventoryView, int64, error) {
	db := r.db.Table("products").
		Select("products.id AS product_id, products.sku AS product_sku, products.name AS product_name, COALESCE(inventory.quantity, 0) AS quantity, COALESCE(inventory.updated_at, products.created_at) AS updated_at").
		Joins("LEFT JOIN inventory ON inventory.product_id = products.id")

	if q.ProductID != uuid.Nil {
		db = db.Where("products.id = ?", q.ProductID)
	}
	if q.Search != "" {
		like := "%" + strings.ToLower(q.Search) + "%"
		db = db.Where("(LOWER(products.name) LIKE ? OR LOWER(products.sku) LIKE ?)", like, like)
	}
	if q.LowStock {
		db = db.Where("COALESCE(inventory.quantity, 0) <= products.low_stock_threshold")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	p, per := normalizePage(q.Page, q.PerPage)
	var views []*InventoryView
	if err := db.Order("products.name ASC").
		Offset((p - 1) * per).
		Limit(per).
		Scan(&views).Error; err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// Transactions returns a filtered, paginated history of stock movements joined
// with product identity. Filters are parameterized and the type value is
// validated before reaching the query builder.
func (r *GORMRepository) Transactions(q TransactionQuery) ([]*TransactionView, int64, error) {
	db := r.db.Table("inventory_transactions AS t").
		Select("t.id, t.product_id, p.sku AS product_sku, p.name AS product_name, t.type, t.quantity, t.unit_cost, t.note, t.user_id, t.created_at").
		Joins("JOIN products p ON p.id = t.product_id")

	if q.ProductID != uuid.Nil {
		db = db.Where("t.product_id = ?", q.ProductID)
	}
	if q.Type != "" && q.Type != "IN" && q.Type != "OUT" {
		return nil, 0, sharederr.ErrValidation
	}
	if q.Type != "" {
		db = db.Where("t.type = ?", q.Type)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	p, per := normalizePage(q.Page, q.PerPage)
	var views []*TransactionView
	if err := db.Order("t.created_at DESC, t.id DESC").
		Offset((p - 1) * per).
		Limit(per).
		Scan(&views).Error; err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// normalizePage clamps page/per-page to the shared defaults and cap.
func normalizePage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}

// isForeignKeyViolation reports whether err is a PostgreSQL FK constraint
// violation (SQLSTATE 23503), e.g. stock movement referencing a deleted product.
func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "violates foreign key constraint")
}
