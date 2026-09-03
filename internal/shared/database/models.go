// Package database - AutoMigrate model registry.
// FK order matters: parent tables before child tables.
package database

import (
	"inventory/internal/activitylog"
	"inventory/internal/adjustment"
	"inventory/internal/cyclecount"
	"inventory/internal/auth"
	"inventory/internal/category"
	"inventory/internal/inventory"
	"inventory/internal/product"
	"inventory/internal/warehouses"
)

// Models returns all GORM models for AutoMigrate in FK-dependency order.
// Order: Role before User; Category before Product; Warehouse before
// Inventory; Product before Inventory/InventoryTransaction;
// User before RefreshToken and ActivityLog.
func Models() []any {
	return []any{
		// Independent parent tables
		&auth.Role{},
		&category.Category{},
		&warehouses.Warehouse{},
		// Permission catalog + role mapping (authority comes from Role)
		&auth.Permission{},
		&auth.RolePermission{},
		// User depends on Role
		&auth.User{},
		// Product depends on Category
		&product.Product{},
		// Inventory tables depend on Product and Warehouse
		&inventory.Inventory{},
		&inventory.LedgerEntry{},
		&inventory.Reservation{},
		// RefreshToken depends on User
		&auth.RefreshToken{},
		// ActivityLog depends on User
		&activitylog.ActivityLog{},
		// Adjustments depend on Product and Warehouse
		&adjustment.Adjustment{},
		&adjustment.SystemSetting{},
		// Cycle counts depend on Warehouse and Product
		&cyclecount.Plan{},
		&cyclecount.Item{},
	}
}
