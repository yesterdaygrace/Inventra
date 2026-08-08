package dashboard

import (
	"context"
	"testing"
	"time"

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

// seedCategory creates a category and returns it.
func seedCategory(t *testing.T, db *gorm.DB, name string) category.Category {
	t.Helper()
	cat := category.Category{Name: name}
	require.NoError(t, db.Create(&cat).Error)
	return cat
}

func seedProduct(t *testing.T, db *gorm.DB, catID uuid.UUID, name, sku string, price float64, threshold int) product.Product {
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
	require.NoError(t, db.Create(&inventory.InventoryTransaction{ProductID: pid, Type: typ, Quantity: qty, UnitCost: unitCost}).Error)
}

func TestRepoCountsAndValue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	cat := seedCategory(t, db, "Electronics")
	laptop := seedProduct(t, db, cat.ID, "Laptop", "LAP", 1000, 5)
	mouse := seedProduct(t, db, cat.ID, "Mouse", "MOU", 20, 5)
	seedQuantity(t, db, laptop.ID, 2)
	seedQuantity(t, db, mouse.ID, 1)

	products, err := repo.CountProducts(context.Background(), )
	require.NoError(t, err)
	assert.Equal(t, int64(2), products)

	categories, err := repo.CountCategories(context.Background(), )
	require.NoError(t, err)
	assert.Equal(t, int64(1), categories)

	value, err := repo.InventoryValue(context.Background(), )
	require.NoError(t, err)
	// No IN cost recorded yet: quantity * list price fallback.
	assert.Equal(t, 2020.0, value)
}

func TestRepoInventoryValueUsesLastStockInCost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	cat := seedCategory(t, db, "Electronics")
	p := seedProduct(t, db, cat.ID, "Widget", "SKU", 1000, 5)
	seedQuantity(t, db, p.ID, 10)
	unitCost := 50.0
	seedMovement(t, db, p.ID, "IN", 10, &unitCost)

	value, err := repo.InventoryValue(context.Background(), )
	require.NoError(t, err)
	assert.Equal(t, 500.0, value)
}

func TestRepoLowStockItems(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	cat := seedCategory(t, db, "Electronics")
	low := seedProduct(t, db, cat.ID, "Low Widget", "LOW", 10, 8)
	high := seedProduct(t, db, cat.ID, "High Stock", "HIGH", 10, 8)
	seedQuantity(t, db, low.ID, 3)
	seedQuantity(t, db, high.ID, 20)

	items, err := repo.LowStockItems(context.Background(), )
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Low Widget", items[0].Name)
	assert.Equal(t, 3, items[0].Quantity)
}

func TestRepoStockHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	cat := seedCategory(t, db, "Electronics")
	healthy := seedProduct(t, db, cat.ID, "Healthy", "H", 1, 5)
	low := seedProduct(t, db, cat.ID, "Low", "L", 1, 5)
	seedProduct(t, db, cat.ID, "Critical", "C", 1, 5) // no inventory row -> counted critical
	seedQuantity(t, db, healthy.ID, 50)
	seedQuantity(t, db, low.ID, 3)

	health, err := repo.StockHealth(context.Background(), )
	require.NoError(t, err)
	assert.Equal(t, int64(1), health.Healthy)
	assert.Equal(t, int64(1), health.Low)
	assert.Equal(t, int64(1), health.Critical)
}

func TestRepoTopSellersAndDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	bookCat := seedCategory(t, db, "Books")
	elecCat := seedCategory(t, db, "Electronics")
	p1 := seedProduct(t, db, bookCat.ID, "Novel", "B1", 10, 5)
	p2 := seedProduct(t, db, elecCat.ID, "Router", "E1", 80, 5)
	seedMovement(t, db, p1.ID, "OUT", 7, nil)
	seedMovement(t, db, p2.ID, "OUT", 3, nil)

	sellers, err := repo.TopSellers(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, sellers, 1)
	assert.Equal(t, "Novel", sellers[0].Name)
	assert.Equal(t, 7, sellers[0].UnitsSold)

	dist, err := repo.CategoryDistribution(context.Background(), )
	require.NoError(t, err)
	require.Len(t, dist, 2)
	assert.Equal(t, "Books", dist[0].Name)
	assert.Equal(t, int64(1), dist[0].Count)
	assert.Equal(t, "Electronics", dist[1].Name)
	assert.Equal(t, int64(1), dist[1].Count)
}

func TestRepoRecentActivitiesAndMovement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	role := auth.Role{Name: "ADMIN"}
	require.NoError(t, db.Create(&role).Error)
	uid := "prod-1"
	eid := "e-1"
	require.NoError(t, db.Create(&activitylog.ActivityLog{
		Action: "CREATE", EntityType: "product", EntityID: &eid, UserID: nil, IP: &uid,
	}).Error)

	recent, err := repo.RecentActivities(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, recent, 1)
	assert.Equal(t, "CREATE", recent[0].Action)
	assert.Equal(t, "product", recent[0].EntityType)

	cat := seedCategory(t, db, "Electronics")
	p := seedProduct(t, db, cat.ID, "Widget", "W", 10, 5)
	seedMovement(t, db, p.ID, "IN", 10, nil)
	seedMovement(t, db, p.ID, "OUT", 4, nil)

	since := time.Now().AddDate(0, 0, -1)
	moves, err := repo.InventoryMovement(context.Background(), since)
	require.NoError(t, err)
	require.Len(t, moves, 1)
	assert.Equal(t, 10, moves[0].StockIn)
	assert.Equal(t, 4, moves[0].StockOut)
}

func TestRepoTotalQuantity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	cat := seedCategory(t, db, "Electronics")
	p1 := seedProduct(t, db, cat.ID, "A", "A1", 1, 5)
	p2 := seedProduct(t, db, cat.ID, "B", "B1", 1, 5)
	seedQuantity(t, db, p1.ID, 3)
	seedQuantity(t, db, p2.ID, 7)

	total, err := repo.TotalQuantity(context.Background(), )
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
}
