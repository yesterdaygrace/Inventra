package inventory

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"inventory/internal/category"
	"inventory/internal/product"
	"inventory/internal/warehouses"
)

var testModels = []any{
	&warehouses.Warehouse{},
	&category.Category{},
	&product.Product{},
	&Inventory{},
	&InventoryTransaction{},
}

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := "host=localhost user=postgres password=postgres dbname=inventory port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS activity_logs CASCADE")
		db.Exec("DROP TABLE IF EXISTS refresh_tokens CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory_transactions CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory CASCADE")
		db.Exec("DROP TABLE IF EXISTS products CASCADE")
		db.Exec("DROP TABLE IF EXISTS categories CASCADE")
		db.Exec("DROP TABLE IF EXISTS warehouses CASCADE")
		db.Exec("DROP TABLE IF EXISTS users CASCADE")
		db.Exec("DROP TABLE IF EXISTS roles CASCADE")
	})

	return db
}

func TestInventory_AutoMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)

	// When: AutoMigrate with testModels
	err := db.AutoMigrate(testModels...)

	// Then
	require.NoError(t, err)

	// Verify inventory table exists
	var invExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables " +
		"WHERE table_schema = 'public' AND table_name = 'inventory')").
		Scan(&invExists).Error
	require.NoError(t, err)
	assert.True(t, invExists, "inventory table should exist")

	// Verify inventory_transactions table exists
	var txnExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables " +
		"WHERE table_schema = 'public' AND table_name = 'inventory_transactions')").
		Scan(&txnExists).Error
	require.NoError(t, err)
	assert.True(t, txnExists, "inventory_transactions table should exist")

	// Verify unique index on inventory.product_id
	var uniqueExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE tablename = 'inventory' 
			AND indexdef LIKE '%product_id%' 
			AND indexdef LIKE '%UNIQUE%'
		)
	`).Scan(&uniqueExists).Error
	require.NoError(t, err)
	assert.True(t, uniqueExists, "unique index on product_id should exist")
}

func TestInventory_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)
	require.NoError(t, err)

	wh := warehouses.Warehouse{Code: "WH1", Name: "Warehouse 1"}
	err = db.Create(&wh).Error
	require.NoError(t, err)

	cat := category.Category{Name: "Electronics"}
	err = db.Create(&cat).Error
	require.NoError(t, err)

	prod := product.Product{
		Name:       "Test Product",
		SKU:        "TEST-001",
		Price:      100.00,
		CategoryID: cat.ID,
	}
	err = db.Create(&prod).Error
	require.NoError(t, err)

	inv := Inventory{
		ProductID:   prod.ID,
		WarehouseID: wh.ID,
		Quantity:    50,
	}
	err = db.Create(&inv).Error

	// Then
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, inv.ID)
	assert.Equal(t, prod.ID, inv.ProductID)
	assert.Equal(t, wh.ID, inv.WarehouseID)
	assert.Equal(t, 50, inv.Quantity)
	assert.False(t, inv.UpdatedAt.IsZero())
}

func TestInventory_UniqueProductWarehousePair(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)
	require.NoError(t, err)

	wh := warehouses.Warehouse{Code: "WH1", Name: "Warehouse 1"}
	err = db.Create(&wh).Error
	require.NoError(t, err)

	wh2 := warehouses.Warehouse{Code: "WH2", Name: "Warehouse 2"}
	err = db.Create(&wh2).Error
	require.NoError(t, err)

	cat := category.Category{Name: "Electronics"}
	err = db.Create(&cat).Error
	require.NoError(t, err)

	prod := product.Product{
		Name:       "Test Product",
		SKU:        "TEST-002",
		Price:      100.00,
		CategoryID: cat.ID,
	}
	err = db.Create(&prod).Error
	require.NoError(t, err)

	// First row: same product in warehouse 1
	err = db.Create(&Inventory{ProductID: prod.ID, WarehouseID: wh.ID, Quantity: 10}).Error
	require.NoError(t, err)

	// Same product in warehouse 2: allowed (different warehouse)
	err = db.Create(&Inventory{ProductID: prod.ID, WarehouseID: wh2.ID, Quantity: 20}).Error
	require.NoError(t, err, "same product in different warehouse should be allowed")

	// Same product in warehouse 1 again: conflict (composite unique violation)
	err = db.Create(&Inventory{ProductID: prod.ID, WarehouseID: wh.ID, Quantity: 30}).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key", "same (product, warehouse) pair must be unique")
}

func TestInventoryTransaction_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)
	require.NoError(t, err)

	cat := category.Category{Name: "Electronics"}
	err = db.Create(&cat).Error
	require.NoError(t, err)

	prod := product.Product{
		Name:       "Test Product",
		SKU:        "TEST-003",
		Price:      100.00,
		CategoryID: cat.ID,
	}
	err = db.Create(&prod).Error
	require.NoError(t, err)

	// When
	unitCost := 80.00
	note := "Initial stock"
	txn := InventoryTransaction{
		ProductID: prod.ID,
		Type:      "IN",
		Quantity:  100,
		UnitCost:  &unitCost,
		Note:      &note,
	}
	err = db.Create(&txn).Error

	// Then
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, txn.ID)
	assert.Equal(t, prod.ID, txn.ProductID)
	assert.Equal(t, "IN", txn.Type)
	assert.Equal(t, 100, txn.Quantity)
	require.NotNil(t, txn.UnitCost)
	assert.Equal(t, 80.00, *txn.UnitCost)
	require.NotNil(t, txn.Note)
	assert.Equal(t, "Initial stock", *txn.Note)
	assert.False(t, txn.CreatedAt.IsZero())
}

func TestInventory_TableName(t *testing.T) {
	// Given
	inv := Inventory{}

	// When
	tableName := inv.TableName()

	// Then
	assert.Equal(t, "inventory", tableName)
}

func TestInventoryTransaction_TableName(t *testing.T) {
	// Given
	txn := InventoryTransaction{}

	// When
	tableName := txn.TableName()

	// Then
	assert.Equal(t, "inventory_transactions", tableName)
}
