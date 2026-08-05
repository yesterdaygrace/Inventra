// Package dashboard computes read-model aggregates for the summary cards
// and chart endpoints. All aggregates are derived on read (no caching).
package dashboard

import "github.com/google/uuid"

// LowStockItem is a product at or below its restock threshold.
type LowStockItem struct {
	ProductID uuid.UUID `gorm:"column:product_id" json:"product_id"`
	SKU       string    `gorm:"column:sku"         json:"sku"`
	Name      string    `gorm:"column:name"        json:"name"`
	Quantity  int       `gorm:"column:quantity"    json:"quantity"`
	Threshold int       `gorm:"column:low_stock_threshold" json:"low_stock_threshold"`
}

// RecentActivity is one audit event surfaced on the dashboard widget.
type RecentActivity struct {
	ID         uuid.UUID  `gorm:"column:id"         json:"id"`
	UserID     *uuid.UUID `gorm:"column:user_id"    json:"user_id,omitempty"`
	UserName   *string    `gorm:"column:user_name"  json:"user_name,omitempty"`
	Action     string     `gorm:"column:action"     json:"action"`
	EntityType string     `gorm:"column:entity_type" json:"entity_type"`
	EntityID   *string    `gorm:"column:entity_id"  json:"entity_id,omitempty"`
	CreatedAt  string     `gorm:"column:created_at" json:"created_at"`
}

// TopSeller aggregates OUT quantity per product for the top-selling widget.
type TopSeller struct {
	ProductID uuid.UUID `gorm:"column:product_id" json:"product_id"`
	SKU       string    `gorm:"column:sku"         json:"sku"`
	Name      string    `gorm:"column:name"        json:"name"`
	UnitsSold int       `gorm:"column:units_sold"  json:"units_sold"`
}

// DayMovement is the STOCK_IN/STOCK_OUT total for one calendar day.
type DayMovement struct {
	Day      string `gorm:"column:day"      json:"day"`
	StockIn  int    `gorm:"column:stock_in" json:"stock_in"`
	StockOut int    `gorm:"column:stock_out" json:"stock_out"`
}

// CategoryCount is a product count per category.
type CategoryCount struct {
	Name  string `gorm:"column:name" json:"name"`
	Count int64  `gorm:"column:count" json:"count"`
}

// StockHealth buckets products by stock level relative to their threshold.
type StockHealth struct {
	Healthy  int64 `json:"healthy"`
	Low      int64 `json:"low"`
	Critical int64 `json:"critical"`
}
