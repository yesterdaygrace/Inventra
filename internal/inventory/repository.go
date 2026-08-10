// GORM-backed implementation of the inventory Repository interface.
package inventory

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"inventory/internal/shared/dbutil"
	sharederr "inventory/internal/shared/errors"
)

// defaultWarehouseCode is the code of the seeded fallback warehouse used when
// a stock movement does not specify a warehouse.
const defaultWarehouseCode = "DEFAULT"

// GORMRepository persists inventory using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// DefaultWarehouse returns the ID of the seeded DEFAULT warehouse. It errors
// with ErrNotFound when the seed has not run yet.
func (r *GORMRepository) DefaultWarehouse(ctx context.Context) (uuid.UUID, error) {
	var row struct {
		ID uuid.UUID
	}
	err := r.db.WithContext(ctx).Table("warehouses").
		Select("id").Where("code = ?", defaultWarehouseCode).Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return uuid.Nil, sharederr.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return row.ID, nil
}

// resolveWarehouse maps a movement's optional warehouse to a concrete ID,
// falling back to the seeded DEFAULT warehouse when omitted.
func (r *GORMRepository) resolveWarehouse(ctx context.Context, wh *uuid.UUID) (uuid.UUID, error) {
	if wh != nil {
		return *wh, nil
	}
	return r.DefaultWarehouse(ctx)
}

// StockIn records an IN movement and increments the product quantity in a
// single DB transaction, scoped to the movement's warehouse (DEFAULT when
// omitted). The inventory row is upserted per (product, warehouse) pair and
// products with no existing row start from zero, so the first IN creates it.
func (r *GORMRepository) StockIn(ctx context.Context, m Movement) (*Inventory, error) {
	whID, err := r.resolveWarehouse(ctx, m.WarehouseID)
	if err != nil {
		return nil, err
	}

	var result Inventory
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", m.ProductID, whID).First(&inv).Error
		switch err {
		case nil:
			inv.Quantity += m.Quantity
		case gorm.ErrRecordNotFound:
			inv = Inventory{ProductID: m.ProductID, WarehouseID: whID, Quantity: m.Quantity}
		default:
			return err
		}

		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		if err := tx.Create(&InventoryTransaction{
			ProductID:   m.ProductID,
			Type:        "IN",
			Quantity:    m.Quantity,
			UnitCost:    m.UnitCost,
			Note:        m.Note,
			UserID:      m.UserID,
			WarehouseID: &whID,
		}).Error; err != nil {
			return err
		}
		result = inv
		return nil
	})
	if err != nil {
		if dbutil.IsForeignKeyViolation(err) {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &result, nil
}

// StockOut records an OUT movement and decrements the product quantity in a
// single transaction, scoped to the movement's warehouse (DEFAULT when
// omitted). It rejects any draw that would push stock below zero, returning
// ErrConflict and rolling back so no partial history row remains.
func (r *GORMRepository) StockOut(ctx context.Context, m Movement) (*Inventory, error) {
	whID, err := r.resolveWarehouse(ctx, m.WarehouseID)
	if err != nil {
		return nil, err
	}

	var result Inventory
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", m.ProductID, whID).First(&inv).Error
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
			ProductID:   m.ProductID,
			Type:        "OUT",
			Quantity:    m.Quantity,
			UnitCost:    m.UnitCost,
			Note:        m.Note,
			UserID:      m.UserID,
			WarehouseID: &whID,
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

// Transfer moves stock between two warehouses in a single transaction: it
// locks and decrements the source row, upserts the destination row, and
// writes two history rows (OUT from source, IN to destination) sharing one
// transfer_id. 404 when the product or either warehouse does not exist; 409
// when the source lacks the requested quantity.
func (r *GORMRepository) Transfer(ctx context.Context, t Transfer) (*Inventory, error) {
	transferID := uuid.New()
	var result Inventory

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Existence checks → 404 semantics.
		var prodCount int64
		if err := tx.Table("products").Where("id = ?", t.ProductID).Count(&prodCount).Error; err != nil {
			return err
		}
		if prodCount == 0 {
			return sharederr.ErrNotFound
		}
		for _, wh := range []uuid.UUID{t.FromWarehouseID, t.ToWarehouseID} {
			var whCount int64
			if err := tx.Table("warehouses").Where("id = ?", wh).Count(&whCount).Error; err != nil {
				return err
			}
			if whCount == 0 {
				return sharederr.ErrNotFound
			}
		}

		// Lock and decrement the source row.
		var src Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", t.ProductID, t.FromWarehouseID).First(&src).Error
		if err == gorm.ErrRecordNotFound {
			return sharederr.ErrConflict
		}
		if err != nil {
			return err
		}
		if src.Quantity < t.Quantity {
			return sharederr.ErrConflict
		}
		src.Quantity -= t.Quantity
		if err := tx.Save(&src).Error; err != nil {
			return err
		}

		// Upsert the destination row (lock, then increment or create).
		var dst Inventory
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", t.ProductID, t.ToWarehouseID).First(&dst).Error
		switch err {
		case nil:
			dst.Quantity += t.Quantity
			if err := tx.Save(&dst).Error; err != nil {
				return err
			}
		case gorm.ErrRecordNotFound:
			dst = Inventory{ProductID: t.ProductID, WarehouseID: t.ToWarehouseID, Quantity: t.Quantity}
			if err := tx.Create(&dst).Error; err != nil {
				return err
			}
		default:
			return err
		}

		// Two history rows sharing one transfer_id.
		if err := tx.Create(&InventoryTransaction{
			ProductID:   t.ProductID,
			Type:        "OUT",
			Quantity:    t.Quantity,
			Note:        t.Note,
			UserID:      t.UserID,
			WarehouseID: &t.FromWarehouseID,
			TransferID:  &transferID,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&InventoryTransaction{
			ProductID:   t.ProductID,
			Type:        "IN",
			Quantity:    t.Quantity,
			Note:        t.Note,
			UserID:      t.UserID,
			WarehouseID: &t.ToWarehouseID,
			TransferID:  &transferID,
		}).Error; err != nil {
			return err
		}

		result = dst
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns a filtered, sorted, paginated joined inventory view (every
// product left-joined with its stock rows) plus the total match count. All
// dynamic values are parameterized, so input cannot be injected. Without a
// warehouse filter quantities are aggregated across all warehouses; with one,
// the view is scoped to that warehouse's rows.
func (r *GORMRepository) List(ctx context.Context, q ListQuery) ([]*InventoryView, int64, error) {
	join := "LEFT JOIN inventory ON inventory.product_id = products.id"
	joinArgs := []any{}
	if q.WarehouseID != nil {
		join += " AND inventory.warehouse_id = ?"
		joinArgs = append(joinArgs, *q.WarehouseID)
	}

	base := r.db.WithContext(ctx).Table("products").Joins(join, joinArgs...)

	if q.ProductID != uuid.Nil {
		base = base.Where("products.id = ?", q.ProductID)
	}
	if q.Search != "" {
		like := "%" + strings.ToLower(q.Search) + "%"
		base = base.Where("(LOWER(products.name) LIKE ? OR LOWER(products.sku) LIKE ?)", like, like)
	}

	// Total: count distinct product rows matching filters (products may join
	// to multiple inventory rows per warehouse).
	grouped := base.Select("products.id").Group("products.id")
	if q.LowStock {
		grouped = grouped.Having("COALESCE(SUM(inventory.quantity), 0) <= products.low_stock_threshold")
	}
	var total int64
	if err := r.db.WithContext(ctx).Table("(?) AS filtered", grouped).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Page data: aggregate quantity per product.
	db := base.Select(strings.Join([]string{
		"products.id AS product_id",
		"products.sku AS product_sku",
		"products.name AS product_name",
		"COALESCE(SUM(inventory.quantity), 0) AS quantity",
		"COALESCE(MAX(inventory.updated_at), products.created_at) AS updated_at",
	}, ", ")).
		Group("products.id, products.sku, products.name, products.created_at")
	if q.LowStock {
		db = db.Having("COALESCE(SUM(inventory.quantity), 0) <= products.low_stock_threshold")
	}

	p, per := dbutil.NormalizePage(q.Page, q.PerPage)
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
func (r *GORMRepository) Transactions(ctx context.Context, q TransactionQuery) ([]*TransactionView, int64, error) {
	db := r.db.WithContext(ctx).Table("inventory_transactions AS t").
		Select(strings.Join([]string{
			"t.id", "t.product_id", "p.sku AS product_sku",
			"p.name AS product_name", "t.type", "t.quantity",
			"t.unit_cost", "t.note", "t.user_id", "t.warehouse_id", "t.transfer_id", "t.created_at",
		}, ", ")).
		Joins("JOIN products p ON p.id = t.product_id")

	if q.ProductID != uuid.Nil {
		db = db.Where("t.product_id = ?", q.ProductID)
	}
	if q.WarehouseID != nil {
		db = db.Where("t.warehouse_id = ?", *q.WarehouseID)
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

	p, per := dbutil.NormalizePage(q.Page, q.PerPage)
	var views []*TransactionView
	if err := db.Order("t.created_at DESC, t.id DESC").
		Offset((p - 1) * per).
		Limit(per).
		Scan(&views).Error; err != nil {
		return nil, 0, err
	}
	return views, total, nil
}
