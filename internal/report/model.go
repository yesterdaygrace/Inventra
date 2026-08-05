// Package report implements lightweight read-model reporting over the
// existing product/inventory aggregates: a per-category stock summary
// (totals plus low-stock items) exposed as an envelope and as CSV.
package report

// StockSummary is the aggregated view returned by the summary report.
type StockSummary struct {
	Categories    []*CategorySummary `json:"categories"`
	LowStock      []*LowStockItem    `json:"low_stock"`
	TotalProducts int64              `json:"total_products"`
	TotalValue    float64            `json:"total_value"`
}

// CategorySummary aggregates stock totals for a single category.
type CategorySummary struct {
	Name         string  `gorm:"column:name"          json:"name"`
	ProductCount int64   `gorm:"column:product_count" json:"product_count"`
	TotalQty     int64   `gorm:"column:total_qty"     json:"total_qty"`
	TotalValue   float64 `gorm:"column:total_value"   json:"total_value"`
}

// LowStockItem is a product whose current quantity is at or below its
// low-stock threshold, plus the category it belongs to.
type LowStockItem struct {
	ProductID string  `gorm:"column:product_id" json:"product_id"`
	SKU       string  `gorm:"column:sku"        json:"sku"`
	Name      string  `gorm:"column:name"       json:"name"`
	Category  string  `gorm:"column:category"   json:"category"`
	Quantity  int     `gorm:"column:quantity"   json:"quantity"`
	Threshold int     `gorm:"column:low_stock_threshold" json:"threshold"`
	Value     float64 `gorm:"column:value"      json:"value"`
}
