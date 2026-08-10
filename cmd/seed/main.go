// Command seed populates the database with base reference data
// (roles and a default admin user) idempotently. It connects to the
// configured PostgreSQL instance, runs AutoMigrate, and inserts rows
// only when absent so it is safe to run repeatedly.
package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"inventory/internal/auth"
	"inventory/internal/inventory"
	"inventory/internal/shared/config"
	"inventory/internal/shared/database"
	"inventory/internal/warehouses"
)

const (
	adminEmail    = "admin@inventory.local"
	adminPassword = "Admin123!"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "demo" {
		runWithDB(func(db *gorm.DB) error {
			return runDemo(db)
		}, "demo")
		return
	}
	if err := run(); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

// runWithDB connects, migrates, then invokes fn against the DB.
func runWithDB(fn func(*gorm.DB) error, note string) {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: load config: %v", note, err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("%s: connect: %v", note, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("%s: sql handle: %v", note, err)
	}
	defer func() {
		if cerr := sqlDB.Close(); cerr != nil {
			log.Printf("%s: close sql connection: %v", note, cerr)
		}
	}()

	if err := database.AutoMigrate(db, database.Models()...); err != nil {
		log.Fatalf("%s: migrate: %v", note, err)
	}
	if err := fn(db); err != nil {
		log.Fatalf("%s: %v", note, err)
	}
	log.Printf("%s data seeding complete", note)
}

// run loads configuration, connects to the database, migrates, and
// seeds roles plus the default admin user idempotently.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("sql handle: %w", err)
	}
	defer func() {
		if cerr := sqlDB.Close(); cerr != nil {
			log.Printf("demo: close sql connection: %v", cerr)
		}
	}()

	if err := database.AutoMigrate(db, database.Models()...); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	fmt.Println("migration complete")

	if err := seedDefaultWarehouse(db); err != nil {
		return err
	}
	if err := seedRoles(db); err != nil {
		return err
	}
	if err := seedAdmin(db, cfg.BCryptCost); err != nil {
		return err
	}

	return nil
}

// seedDefaultWarehouse creates the DEFAULT warehouse idempotently and
// backfills any inventory rows that predate multi-warehouse support so every
// row resolves to it. Safe to run repeatedly.
func seedDefaultWarehouse(db *gorm.DB) error {
	wh := warehouses.Warehouse{
		Code:        "DEFAULT",
		Name:        "Default Warehouse",
		Description: strPtr("Fallback warehouse for legacy single-location stock"),
		IsActive:    true,
	}
	if err := db.Where(warehouses.Warehouse{Code: "DEFAULT"}).FirstOrCreate(&wh).Error; err != nil {
		return fmt.Errorf("seed default warehouse: %w", err)
	}

	if res := db.Model(&inventory.Inventory{}).
		Where("warehouse_id IS NULL").
		Update("warehouse_id", wh.ID); res.Error != nil {
		return fmt.Errorf("backfill inventory warehouse_id: %w", res.Error)
	}
	if res := db.Model(&inventory.InventoryTransaction{}).
		Where("warehouse_id IS NULL").
		Update("warehouse_id", wh.ID); res.Error != nil {
		return fmt.Errorf("backfill inventory_transactions warehouse_id: %w", res.Error)
	}

	fmt.Printf("seeded default warehouse %s (%s)\n", wh.Code, wh.ID)
	return nil
}

func strPtr(s string) *string { return &s }

// seedRoles inserts ADMIN and STAFF roles if they do not already exist.
func seedRoles(db *gorm.DB) error {
	roles := []struct{ name string }{
		{name: "ADMIN"},
		{name: "STAFF"},
	}
	for _, r := range roles {
		role := auth.Role{Name: r.name}
		if err := db.FirstOrCreate(&role, auth.Role{Name: r.name}).Error; err != nil {
			return fmt.Errorf("seed role %s: %w", r.name, err)
		}
	}
	return nil
}

// seedAdmin inserts the default admin user if one does not already exist.
func seedAdmin(db *gorm.DB, cost int) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), cost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	role := auth.Role{}
	if err := db.Where(auth.Role{Name: "ADMIN"}).First(&role).Error; err != nil {
		return fmt.Errorf("find ADMIN role: %w", err)
	}

	user := auth.User{
		Name:         "System Administrator",
		Email:        adminEmail,
		PasswordHash: string(hash),
		RoleID:       role.ID,
		IsActive:     true,
	}

	var count int64
	if err := db.Model(&auth.User{}).Where("email = ?", adminEmail).Count(&count).Error; err != nil {
		return fmt.Errorf("count admin: %w", err)
	}
	if count > 0 {
		fmt.Printf("admin user %s already exists, skipping\n", adminEmail)
		return nil
	}
	if err := db.Create(&user).Error; err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	fmt.Printf("seeded admin user %s (role ADMIN)\n", adminEmail)
	return nil
}
