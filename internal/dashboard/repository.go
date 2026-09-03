// GORM-backed implementation of the dashboard Repository interface.
package dashboard

import (
	"context"
	"time"

	"gorm.io/gorm"

	"inventory/internal/shared/dbutil"
)

// GORMRepository computes dashboard aggregates over the shared tables.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// CountProducts returns the number of non-archived products.
func (r *GORMRepository) CountProducts(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("products").Where("is_archived = ?", false).Count(&count).Error
	return count, err
}

// TotalQuantity returns the sum of all current inventory quantities.
func (r *GORMRepository) TotalQuantity(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Raw(`SELECT COALESCE(SUM(quantity), 0) FROM inventory`).Scan(&total).Error
	return total, err
}

// CountCategories returns the number of active categories.
func (r *GORMRepository) CountCategories(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("categories").Where("is_active = ?", true).Count(&count).Error
	return count, err
}

// InventoryValue sums quantity * cost of each product, using the most recent
// IN unit cost when available and the list price otherwise. Products without
// an inventory row contribute zero.
func (r *GORMRepository) InventoryValue(ctx context.Context) (float64, error) {
	var value float64
	err := r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(
			COALESCE(i.quantity, 0) *
			COALESCE((
				SELECT t.unit_cost FROM inventory_ledger t
				WHERE t.product_id = p.id AND t.transaction_type = 'RECEIVE' AND t.unit_cost IS NOT NULL
				ORDER BY t.created_at DESC, t.id DESC LIMIT 1
			), p.price)
		), 0)
		FROM products p
		LEFT JOIN inventory i ON i.product_id = p.id
		WHERE p.is_archived = ?`, false).Scan(&value).Error
	return value, err
}

// LowStockItems lists non-archived products at or below their threshold.
func (r *GORMRepository) LowStockItems(ctx context.Context) ([]*LowStockItem, error) {
	var items []*LowStockItem
	err := r.db.WithContext(ctx).Raw(`
		SELECT p.id AS product_id, p.sku, p.name,
			COALESCE(i.quantity, 0) AS quantity, p.low_stock_threshold
		FROM products p
		LEFT JOIN inventory i ON i.product_id = p.id
		WHERE p.is_archived = ? AND COALESCE(i.quantity, 0) <= p.low_stock_threshold
		ORDER BY COALESCE(i.quantity, 0) ASC, p.name ASC`, false).
		Scan(&items).Error
	return items, err
}

// RecentActivities returns the latest audit events for the dashboard widget.
func (r *GORMRepository) RecentActivities(ctx context.Context, limit int) ([]*RecentActivity, error) {
	var items []*RecentActivity
	err := r.db.WithContext(ctx).Raw(`
		SELECT l.id, l.user_id, u.name AS user_name, l.action, l.entity_type,
			l.entity_id, l.created_at
		FROM activity_logs l
		LEFT JOIN users u ON u.id = l.user_id
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT ?`, limit).Scan(&items).Error
	return items, err
}

// Activities returns a paginated, newest-first page of audit events plus the
// total count. Backs the documented GET /dashboard/activity feed.
func (r *GORMRepository) Activities(ctx context.Context, page, perPage int) ([]*RecentActivity, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM activity_logs`).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	p, per := dbutil.NormalizePage(page, perPage)
	var items []*RecentActivity
	err := r.db.WithContext(ctx).Raw(`
		SELECT l.id, l.user_id, u.name AS user_name, l.action, l.entity_type,
			l.entity_id, l.created_at
		FROM activity_logs l
		LEFT JOIN users u ON u.id = l.user_id
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT ? OFFSET ?`, per, (p-1)*per).Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// TopSellers aggregates OUT movements per product, most sold first.
func (r *GORMRepository) TopSellers(ctx context.Context, limit int) ([]*TopSeller, error) {
	var items []*TopSeller
	err := r.db.WithContext(ctx).Raw(`
		SELECT t.product_id, p.sku, p.name, SUM(t.quantity) AS units_sold
		FROM inventory_ledger t
		JOIN products p ON p.id = t.product_id
		WHERE t.transaction_type = 'ISSUE'
		GROUP BY t.product_id, p.sku, p.name
		ORDER BY units_sold DESC, p.name ASC
		LIMIT ?`, limit).Scan(&items).Error
	return items, err
}

// InventoryMovement returns per-day IN and OUT totals since the given date.
func (r *GORMRepository) InventoryMovement(ctx context.Context, since time.Time) ([]*DayMovement, error) {
	var items []*DayMovement
	err := r.db.WithContext(ctx).Raw(`
		SELECT to_char(created_at, 'YYYY-MM-DD') AS day,
			COALESCE(SUM(CASE WHEN transaction_type = 'RECEIVE' THEN quantity ELSE 0 END), 0) AS stock_in,
			COALESCE(SUM(CASE WHEN transaction_type = 'ISSUE' THEN quantity ELSE 0 END), 0) AS stock_out
		FROM inventory_ledger
		WHERE created_at >= ?
		GROUP BY to_char(created_at, 'YYYY-MM-DD')
		ORDER BY day ASC`, since).Scan(&items).Error
	return items, err
}

// CategoryDistribution returns the product count per category.
func (r *GORMRepository) CategoryDistribution(ctx context.Context) ([]*CategoryCount, error) {
	var items []*CategoryCount
	req := `
		SELECT c.name AS name, COUNT(p.id) AS count
		FROM categories c
		LEFT JOIN products p ON p.category_id = c.id AND p.is_archived = ?
		WHERE c.is_active = ?
		GROUP BY c.id, c.name
		ORDER BY count DESC, c.name ASC`
	err := r.db.WithContext(ctx).Raw(req, false, true).Scan(&items).Error
	return items, err
}

// StockHealth buckets products by stock level relative to their threshold.
// Products with no inventory row are counted as critical (zero stock).
func (r *GORMRepository) StockHealth(ctx context.Context) (StockHealth, error) {
	var rows []struct {
		Lev string
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT CASE
			WHEN COALESCE(i.quantity, 0) = 0 THEN 'critical'
			WHEN COALESCE(i.quantity, 0) <= p.low_stock_threshold THEN 'low'
			ELSE 'healthy'
		END AS lev
		FROM products p
		LEFT JOIN inventory i ON i.product_id = p.id
		WHERE p.is_archived = ?`, false).Scan(&rows).Error
	if err != nil {
		return StockHealth{}, err
	}
	var sh StockHealth
	for _, row := range rows {
		switch row.Lev {
		case "healthy":
			sh.Healthy++
		case "low":
			sh.Low++
		case "critical":
			sh.Critical++
		}
	}
	return sh, nil
}
