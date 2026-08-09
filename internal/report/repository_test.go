package report

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"inventory/internal/activitylog"
	"inventory/internal/auth"
	"inventory/internal/category"
	"inventory/internal/inventory"
	"inventory/internal/product"
)

var testModels = []any{
	&auth.Role{},
	&auth.User{},
	&category.Category{},
	&product.Product{},
	&inventory.Inventory{},
	&inventory.InventoryTransaction{},
	&activitylog.ActivityLog{},
}

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := "host=localhost user=postgres password=postgres dbname=inventory port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS activity_logs CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory_transactions CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory CASCADE")
		db.Exec("DROP TABLE IF EXISTS products CASCADE")
		db.Exec("DROP TABLE IF EXISTS categories CASCADE")
		db.Exec("DROP TABLE IF EXISTS users CASCADE")
		db.Exec("DROP TABLE IF EXISTS roles CASCADE")
	})

	return db
}

func seedCategory(t *testing.T, db *gorm.DB, name string) category.Category {
	t.Helper()
	cat := category.Category{Name: name}
	require.NoError(t, db.Create(&cat).Error)
	return cat
}

func seedProduct(
	t *testing.T, db *gorm.DB, catID uuid.UUID,
	name, sku string, price float64, threshold int,
) product.Product {
	t.Helper()
	p := product.Product{
		Name:              name,
		SKU:               sku,
		Price:             price,
		CategoryID:        catID,
		LowStockThreshold: threshold,
	}
	require.NoError(t, db.Create(&p).Error)
	return p
}

func seedQuantity(t *testing.T, db *gorm.DB, pid uuid.UUID, qty int) {
	t.Helper()
	require.NoError(t, db.Create(&inventory.Inventory{ProductID: pid, Quantity: qty}).Error)
}

func seedMovement(t *testing.T, db *gorm.DB, pid uuid.UUID, typ string, qty int, unitCost *float64) {
	t.Helper()
	require.NoError(t, db.Create(&inventory.InventoryTransaction{
		ProductID: pid, Type: typ, Quantity: qty, UnitCost: unitCost,
	}).Error)
}

func TestRepoStockSummaryGroupsByCategory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	books := seedCategory(t, db, "Books")
	tools := seedCategory(t, db, "Tools")

	bookA := seedProduct(t, db, books.ID, "Go Handbook", "GOB", 40, 3)
	bookB := seedProduct(t, db, books.ID, "SQL Guide", "SQL", 25, 3)
	tool := seedProduct(t, db, tools.ID, "Hammer", "HAM", 15, 4)

	cost := 20.0
	seedMovement(t, db, bookA.ID, "IN", 2, &cost)
	seedQuantity(t, db, bookA.ID, 5)
	seedQuantity(t, db, tool.ID, 10)
	seedQuantity(t, db, bookB.ID, 1)

	sum, err := repo.StockSummary(context.Background())
	require.NoError(t, err)
	require.Len(t, sum.Categories, 2)

	byName := map[string]*CategorySummary{}
	for _, c := range sum.Categories {
		byName[c.Name] = c
	}

	booksSum := byName["Books"]
	assert.Equal(t, int64(2), booksSum.ProductCount)
	assert.Equal(t, int64(6), booksSum.TotalQty)
	assert.Equal(t, 5*20.0+1*25.0, booksSum.TotalValue)

	toolsSum := byName["Tools"]
	assert.Equal(t, int64(1), toolsSum.ProductCount)
	assert.Equal(t, int64(10), toolsSum.TotalQty)
	assert.Equal(t, 10*15.0, toolsSum.TotalValue)
}

func TestRepoStockSummaryLowStockList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	cat := seedCategory(t, db, "Tools")
	low := seedProduct(t, db, cat.ID, "Nail", "NAI", 1, 5)
	healthy := seedProduct(t, db, cat.ID, "Bolt", "BOL", 2, 5)
	seedQuantity(t, db, low.ID, 2)
	seedQuantity(t, db, healthy.ID, 20)

	sum, err := repo.StockSummary(context.Background())
	require.NoError(t, err)
	require.Len(t, sum.LowStock, 1)
	it := sum.LowStock[0]
	assert.Equal(t, low.ID.String(), it.ProductID)
	assert.Equal(t, "Nail", it.Name)
	assert.Equal(t, "Tools", it.Category)
	assert.Equal(t, 2, it.Quantity)
	assert.Equal(t, 5, it.Threshold)
}

func TestRepoStockSummaryEmptyState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	sum, err := repo.StockSummary(context.Background())
	require.NoError(t, err)
	require.NotNil(t, sum.Categories)
	require.NotNil(t, sum.LowStock)
	assert.Empty(t, sum.Categories)
	assert.Empty(t, sum.LowStock)
}

func TestRepoCountProductsAndValue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	cat := seedCategory(t, db, "Electronics")
	p1 := seedProduct(t, db, cat.ID, "Laptop", "LAP", 1000, 5)
	p2 := seedProduct(t, db, cat.ID, "Mouse", "MOU", 20, 5)
	seedQuantity(t, db, p1.ID, 2)

	count, err := repo.CountProducts(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	value, err := repo.InventoryValue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2*1000.0, value) // falls back to price when no IN cost exists

	cost := 900.0
	seedMovement(t, db, p1.ID, "IN", 2, &cost)
	value, err = repo.InventoryValue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2*900.0, value) // last IN cost overrides price
	_ = p2
}
