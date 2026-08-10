// Demo data seeding, only invoked explicitly via `go run ./cmd/seed demo`
// (or `make seed-demo`). Idempotent: categories are matched by name and
// products by SKU, so repeated runs do not create duplicates.
package main

import (
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"inventory/internal/auth"
	"inventory/internal/category"
	"inventory/internal/inventory"
	"inventory/internal/product"
	"inventory/internal/warehouses"
)

// demoCategories are the 5 categories seeded in runDemo.
var demoCategories = []string{
	"Electronics",
	"Furniture",
	"Office Supplies",
	"Clothing",
	"Accessories",
}

// demoProducts is a small, deterministic set so the UI has realistic data.
var demoProducts = []struct {
	Name  string
	SKU   string
	Price float64
	Cat   string
}{
	{"Wireless Mouse", "ELEC-001", 24.99, "Electronics"},
	{"Mechanical Keyboard", "ELEC-002", 89.99, "Electronics"},
	{"27in Monitor", "ELEC-003", 219.99, "Electronics"},
	{"Webcam 1080p", "ELEC-004", 49.99, "Electronics"},
	{"USB-C Dock", "ELEC-005", 69.99, "Electronics"},
	{"Office Desk", "FURN-001", 349.99, "Furniture"},
	{"Ergonomic Chair", "FURN-002", 299.99, "Furniture"},
	{"Bookshelf", "FURN-003", 129.99, "Furniture"},
	{"Standing Desk", "FURN-004", 499.99, "Furniture"},
	{"Filing Cabinet", "FURN-005", 159.99, "Furniture"},
	{"Notebook Pack", "OFF-001", 9.99, "Office Supplies"},
	{"Pens (Box)", "OFF-002", 14.99, "Office Supplies"},
	{"Stapler", "OFF-003", 7.99, "Office Supplies"},
	{"Paper Ream", "OFF-004", 6.99, "Office Supplies"},
	{"Label Printer", "OFF-005", 119.99, "Office Supplies"},
	{"Cotton T-Shirt", "CLTH-001", 19.99, "Clothing"},
	{"Hoodie", "CLTH-002", 39.99, "Clothing"},
	{"Polo Shirt", "CLTH-003", 24.99, "Clothing"},
	{"Backpack", "ACC-001", 49.99, "Accessories"},
	{"Laptop Sleeve", "ACC-002", 29.99, "Accessories"},
}

func runDemo(db *gorm.DB) error {
	if err := seedDefaultWarehouse(db); err != nil {
		return err
	}
	if err := seedDemoCategories(db); err != nil {
		return err
	}
	if err := seedDemoProducts(db); err != nil {
		return err
	}
	if err := seedDemoInventory(db); err != nil {
		return err
	}
	if err := seedDemoUser(db); err != nil {
		return err
	}
	return nil
}

// seedDemoUser creates the demo STAFF user used by demo auto-login mode
// (POST /auth/demo) if it does not already exist.
func seedDemoUser(db *gorm.DB) error {
	var count int64
	if err := db.Model(&auth.User{}).Where("email = ?", auth.DemoEmail).Count(&count).Error; err != nil {
		return fmt.Errorf("count demo user: %w", err)
	}
	if count > 0 {
		fmt.Printf("demo user %s already exists, skipping\n", auth.DemoEmail)
		return nil
	}

	role := auth.Role{}
	if err := db.Where(auth.Role{Name: "STAFF"}).First(&role).Error; err != nil {
		return fmt.Errorf("find STAFF role: %w", err)
	}

	randomPass, err := bcrypt.GenerateFromPassword([]byte(fmt.Sprintf("demo-%s", uuid.NewString())), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}
	user := auth.User{
		Name:         "Demo User",
		Email:        auth.DemoEmail,
		PasswordHash: string(randomPass),
		RoleID:       role.ID,
		IsActive:     true,
	}
	if err := db.Create(&user).Error; err != nil {
		return fmt.Errorf("create demo user: %w", err)
	}
	fmt.Printf("seeded demo user %s (role STAFF)\n", auth.DemoEmail)
	return nil
}

func seedDemoCategories(db *gorm.DB) error {
	for _, name := range demoCategories {
		var c category.Category
		if err := db.Where(category.Category{Name: name}).FirstOrCreate(&c, category.Category{Name: name}).Error; err != nil {
			return fmt.Errorf("seed category %s: %w", name, err)
		}
	}
	fmt.Println("seeded demo categories")
	return nil
}

func seedDemoProducts(db *gorm.DB) error {
	for _, p := range demoProducts {
		// skip if SKU present (idempotent)
		var existing int64
		if err := db.Model(&product.Product{}).Where("sku = ?", p.SKU).Count(&existing).Error; err != nil {
			return fmt.Errorf("check product %s: %w", p.SKU, err)
		}
		if existing > 0 {
			continue
		}
		var cat category.Category
		if err := db.Where(category.Category{Name: p.Cat}).First(&cat).Error; err != nil {
			return fmt.Errorf("find category %s: %w", p.Cat, err)
		}
		prod := product.Product{
			Name:              p.Name,
			SKU:               p.SKU,
			Description:       &p.Name,
			Price:             p.Price,
			CategoryID:        cat.ID,
			LowStockThreshold: 5,
		}
		if err := db.Create(&prod).Error; err != nil {
			return fmt.Errorf("create product %s: %w", p.SKU, err)
		}
	}
	fmt.Println("seeded demo products")
	return nil
}

func seedDemoInventory(db *gorm.DB) error {
	// Fetch the DEFAULT warehouse so new inventory rows reference it.
	var defaultWH warehouses.Warehouse
	if err := db.Where(warehouses.Warehouse{Code: "DEFAULT"}).First(&defaultWH).Error; err != nil {
		return fmt.Errorf("find default warehouse: %w", err)
	}

	// Give fresh stock lines + OPENING transactions so the UI is non-empty.
	var products []product.Product
	if err := db.Find(&products).Error; err != nil {
		return fmt.Errorf("load products: %w", err)
	}
	for _, prod := range products {
		var existing int64
		if err := db.Model(&inventory.Inventory{}).
			Where("product_id = ? AND warehouse_id = ?", prod.ID, defaultWH.ID).
			Count(&existing).Error; err != nil {
			return fmt.Errorf("check inventory %s: %w", prod.SKU, err)
		}
		if existing > 0 {
			continue
		}
		inv := inventory.Inventory{ProductID: prod.ID, WarehouseID: defaultWH.ID, Quantity: 100}
		if err := db.Create(&inv).Error; err != nil {
			return fmt.Errorf("create inventory %s: %w", prod.SKU, err)
		}
		note := "Initial opening stock"
		txn := inventory.InventoryTransaction{
			ProductID:   prod.ID,
			Type:        "IN",
			Quantity:    100,
			UnitCost:    &prod.Price,
			Note:        &note,
			WarehouseID: &defaultWH.ID,
		}
		if err := db.Create(&txn).Error; err != nil {
			return fmt.Errorf("create opening txn %s: %w", prod.SKU, err)
		}
	}
	return nil
}
