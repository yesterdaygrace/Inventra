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
	if err := seedPermissions(db); err != nil {
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
	if res := db.Model(&inventory.LedgerEntry{}).
		Where("warehouse_id IS NULL").
		Update("warehouse_id", wh.ID); res.Error != nil {
		return fmt.Errorf("backfill inventory_ledger warehouse_id: %w", res.Error)
	}

	fmt.Printf("seeded default warehouse %s (%s)\n", wh.Code, wh.ID)
	return nil
}

func strPtr(s string) *string { return &s }

// seedRoles inserts the four built-in roles if they do not already exist.
func seedRoles(db *gorm.DB) error {
	roles := []struct{ name string }{
		{name: "ADMIN"},
		{name: "WAREHOUSE_MANAGER"},
		{name: "STAFF"},
		{name: "VIEWER"},
	}
	for _, r := range roles {
		role := auth.Role{Name: r.name}
		if err := db.FirstOrCreate(&role, auth.Role{Name: r.name}).Error; err != nil {
			return fmt.Errorf("seed role %s: %w", r.name, err)
		}
	}
	return nil
}

// catalogPermissions lists every permission code with a human description.
// Keep in sync with auth.PermissionSetForRole so the seeded DB and the
// compiled-in role sets can never diverge.
func catalogPermissions() []auth.Permission {
	return []auth.Permission{
		{Code: "product.read", Description: "View products"},
		{Code: "product.create", Description: "Create products"},
		{Code: "product.update", Description: "Update products"},
		{Code: "product.delete", Description: "Delete products"},
		{Code: "category.read", Description: "View categories"},
		{Code: "category.create", Description: "Create categories"},
		{Code: "category.update", Description: "Update categories"},
		{Code: "category.delete", Description: "Delete categories"},
		{Code: "warehouse.read", Description: "View warehouses"},
		{Code: "warehouse.manage", Description: "Create, update, and delete warehouses"},
		{Code: "inventory.read", Description: "View inventory"},
		{Code: "inventory.receive", Description: "Receive stock into warehouses"},
		{Code: "inventory.issue", Description: "Issue stock out of warehouses"},
		{Code: "inventory.adjust", Description: "Submit stock adjustments"},
		{Code: "inventory.transfer", Description: "Transfer stock between warehouses"},
		{Code: "user.manage", Description: "Manage users and role assignments"},
		{Code: "audit.read", Description: "Read the audit log"},
		{Code: "report.read", Description: "View reports"},
		{Code: "report.export", Description: "Export reports to CSV"},
		{Code: "dashboard.read", Description: "View dashboard"},
	}
}

// seedPermissions upserts the permission catalog and rewrites each role's
// grant set idempotently: permissions absent from the role's set are
// removed, missing ones are inserted. Safe to run repeatedly.
func seedPermissions(db *gorm.DB) error {
	for _, perm := range catalogPermissions() {
		var row auth.Permission
		if err := db.Where(auth.Permission{Code: perm.Code}).FirstOrCreate(&row, perm).Error; err != nil {
			return fmt.Errorf("seed permission %s: %w", perm.Code, err)
		}
	}
	roles := []string{"ADMIN", "WAREHOUSE_MANAGER", "STAFF", "VIEWER"}
	for _, name := range roles {
		var role auth.Role
		if err := db.Where(auth.Role{Name: name}).First(&role).Error; err != nil {
			return fmt.Errorf("find role %s: %w", name, err)
		}
		codes := auth.PermissionSetForRole(name)
		if err := db.Exec(`DELETE FROM role_permissions WHERE role_id = ?`, role.ID).Error; err != nil {
			return fmt.Errorf("clear role_permissions %s: %w", name, err)
		}
		for _, code := range codes {
			if err := db.Exec(`
				INSERT INTO role_permissions (role_id, permission_id)
				SELECT ?, id FROM permissions WHERE code = ?`,
				role.ID, code).Error; err != nil {
				return fmt.Errorf("grant %s %s: %w", name, code, err)
			}
		}
		fmt.Printf("seeded %s permissions (%d)\n", name, len(codes))
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
