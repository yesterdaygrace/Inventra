// Package report — GORM-backed queries for the stock summary read-model.
package report

import (
	"gorm.io/gorm"
)

// GORMRepository reads report data directly from the shared tables.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a report repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// StockSummary aggregates products per category with the same last-IN-cost
// valuation used by the dashboard, plus the low-stock list.
func (r *GORMRepository) StockSummary() (*StockSummary, error) {
	var categories []*CategorySummary
	err := r.db.Raw(`
		SELECT c.name AS name,
			COUNT(p.id) AS product_count,
			COALESCE(SUM(i.quantity), 0) AS total_qty,
			COALESCE(SUM(
				COALESCE(i.quantity, 0) *
				COALESCE((
					SELECT t.unit_cost FROM inventory_transactions t
					WHERE t.product_id = p.id AND t.type = 'IN' AND t.unit_cost IS NOT NULL
					ORDER BY t.created_at DESC, t.id DESC LIMIT 1
				), p.price)
			), 0) AS total_value
		FROM categories c
		LEFT JOIN products p ON p.category_id = c.id AND p.is_archived = ?
		LEFT JOIN inventory i ON i.product_id = p.id
		GROUP BY c.id, c.name
		ORDER BY c.name ASC`, false).Scan(&categories).Error
	if err != nil {
		return nil, err
	}

	lowStock, err := r.lowStockItems()
	if err != nil {
		return nil, err
	}
	if lowStock == nil {
		lowStock = []*LowStockItem{}
	}
	if categories == nil {
		categories = []*CategorySummary{}
	}

	return &StockSummary{Categories: categories, LowStock: lowStock}, nil
}

// CountProducts returns the number of non-archived products.
func (r *GORMRepository) CountProducts() (int64, error) {
	var count int64
	err := r.db.Table("products").Where("is_archived = ?", false).Count(&count).Error
	return count, err
}

// InventoryValue sums quantity * cost of each product using the same
// last-IN-cost-with-price-fallback valuation as the dashboard.
func (r *GORMRepository) InventoryValue() (float64, error) {
	var value float64
	err := r.db.Raw(`
		SELECT COALESCE(SUM(
			COALESCE(i.quantity, 0) *
			COALESCE((
				SELECT t.unit_cost FROM inventory_transactions t
				WHERE t.product_id = p.id AND t.type = 'IN' AND t.unit_cost IS NOT NULL
				ORDER BY t.created_at DESC, t.id DESC LIMIT 1
			), p.price)
		), 0)
		FROM products p
		LEFT JOIN inventory i ON i.product_id = p.id
		WHERE p.is_archived = ?`, false).Scan(&value).Error
	return value, err
}

// lowStockItems lists non-archived products at or below their threshold
// joined with their category name.
func (r *GORMRepository) lowStockItems() ([]*LowStockItem, error) {
	var items []*LowStockItem
	err := r.db.Raw(`
		SELECT p.id AS product_id, p.sku, p.name, c.name AS category,
			COALESCE(i.quantity, 0) AS quantity, p.low_stock_threshold,
			COALESCE(i.quantity, 0) *
				COALESCE((
					SELECT t.unit_cost FROM inventory_transactions t
					WHERE t.product_id = p.id AND t.type = 'IN' AND t.unit_cost IS NOT NULL
					ORDER BY t.created_at DESC, t.id DESC LIMIT 1
				), p.price) AS value
		FROM products p
		LEFT JOIN inventory i ON i.product_id = p.id
		JOIN categories c ON c.id = p.category_id
		WHERE p.is_archived = ? AND COALESCE(i.quantity, 0) <= p.low_stock_threshold
		ORDER BY COALESCE(i.quantity, 0) ASC, p.name ASC`, false).
		Scan(&items).Error
	return items, err
}
