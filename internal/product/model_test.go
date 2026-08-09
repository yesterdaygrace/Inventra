package product

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"inventory/internal/category"
)

var testModels = []any{
	&category.Category{},
	&Product{},
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
		db.Exec("DROP TABLE IF EXISTS users CASCADE")
		db.Exec("DROP TABLE IF EXISTS roles CASCADE")
	})

	return db
}

func TestProduct_AutoMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)

	// When: AutoMigrate with testModels
	err := db.AutoMigrate(testModels...)

	// Then
	require.NoError(t, err)

	// Verify products table exists
	var tableExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables " +
		"WHERE table_schema = 'public' AND table_name = 'products')").
		Scan(&tableExists).Error
	require.NoError(t, err)
	assert.True(t, tableExists, "products table should exist")

	// Verify unique index on SKU
	var uniqueExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE tablename = 'products' 
			AND indexdef LIKE '%sku%' 
			AND indexdef LIKE '%UNIQUE%'
		)
	`).Scan(&uniqueExists).Error
	require.NoError(t, err)
	assert.True(t, uniqueExists, "unique index on sku should exist")
}

func TestProduct_Create(t *testing.T) {
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

	// When
	desc := "Test product"
	product := Product{
		Name:              "Laptop",
		SKU:               "LAP-001",
		Description:       &desc,
		Price:             999.99,
		CategoryID:        cat.ID,
		LowStockThreshold: 5,
		IsArchived:        false,
	}
	err = db.Create(&product).Error

	// Then
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, product.ID)
	assert.Equal(t, "Laptop", product.Name)
	assert.Equal(t, "LAP-001", product.SKU)
	assert.Equal(t, 999.99, product.Price)
	assert.Equal(t, 5, product.LowStockThreshold)
	assert.False(t, product.IsArchived)
	assert.False(t, product.CreatedAt.IsZero())
	assert.False(t, product.UpdatedAt.IsZero())
}

func TestProduct_UniqueSKU(t *testing.T) {
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

	product1 := Product{
		Name:       "Product One",
		SKU:        "DUPE-SKU",
		Price:      10.00,
		CategoryID: cat.ID,
	}
	err = db.Create(&product1).Error
	require.NoError(t, err)

	// When: try to create another product with same SKU
	product2 := Product{
		Name:       "Product Two",
		SKU:        "DUPE-SKU",
		Price:      20.00,
		CategoryID: cat.ID,
	}
	err = db.Create(&product2).Error

	// Then: should fail due to unique constraint
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestProduct_CategoryFK(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)
	require.NoError(t, err)

	// When: try to create product with non-existent category_id
	product := Product{
		Name:       "Orphan Product",
		SKU:        "ORPHAN-001",
		Price:      99.99,
		CategoryID: uuid.New(), // Non-existent category
	}
	err = db.Create(&product).Error

	// Then: should fail due to FK constraint
	require.Error(t, err)
	assert.Contains(t, err.Error(), "violates foreign key constraint")
}

func TestProduct_TableName(t *testing.T) {
	// Given
	product := Product{}

	// When
	tableName := product.TableName()

	// Then
	assert.Equal(t, "products", tableName)
}
