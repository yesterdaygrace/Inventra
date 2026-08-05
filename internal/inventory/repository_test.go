package inventory

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"inventory/internal/category"
	"inventory/internal/product"
	sharederr "inventory/internal/shared/errors"
)

// setupForRepo migrates the model set and returns a repository over a clean DB.
func setupForRepo(t *testing.T) (*gorm.DB, *GORMRepository, product.Product) {
	t.Helper()
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&category.Category{}, &product.Product{}, &Inventory{}, &InventoryTransaction{}))

	cat := category.Category{Name: "Electronics"}
	require.NoError(t, db.Create(&cat).Error)
	p := product.Product{Name: "Widget", SKU: "WID-1", Price: 10, CategoryID: cat.ID, LowStockThreshold: 5}
	require.NoError(t, db.Create(&p).Error)

	return db, NewGORMRepository(db), p
}

func countTx(t *testing.T, db *gorm.DB, productID uuid.UUID) int64 {
	t.Helper()
	var c int64
	require.NoError(t, db.Model(&InventoryTransaction{}).Where("product_id = ?", productID).Count(&c).Error)
	return c
}

func TestStockInCreatesRowAndHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	inv, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 15})
	require.NoError(t, err)
	assert.Equal(t, 15, inv.Quantity)
	assert.Equal(t, int64(1), countTx(t, db, p.ID))
}

func TestStockInThenStockOutNetsCorrectQty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	inv1, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 20})
	require.NoError(t, err)
	assert.Equal(t, 20, inv1.Quantity)

	inv2, err := repo.StockOut(Movement{ProductID: p.ID, Type: "OUT", Quantity: 6})
	require.NoError(t, err)
	assert.Equal(t, 14, inv2.Quantity)

	assert.Equal(t, int64(2), countTx(t, db, p.ID))

	var net int64
	require.NoError(t, db.Model(&Inventory{}).Where("product_id = ?", p.ID).Pluck("quantity", &net).Error)
	assert.Equal(t, int64(14), net)
}

func TestStockOutOverdrawRollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)
	_, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 5})
	require.NoError(t, err)

	_, err = repo.StockOut(Movement{ProductID: p.ID, Type: "OUT", Quantity: 100})
	assert.ErrorIs(t, err, sharederr.ErrConflict)

	var qty int64
	require.NoError(t, db.Model(&Inventory{}).Where("product_id = ?", p.ID).Pluck("quantity", &qty).Error)
	assert.Equal(t, int64(5), qty, "quantity unchanged after overdraw")

	assert.Equal(t, int64(1), countTx(t, db, p.ID), "no partial OUT history row")
}

func TestStockOutNoRowConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)

	_, err := repo.StockOut(Movement{ProductID: p.ID, Type: "OUT", Quantity: 1})
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestStockInHistoryHasTypeAndQty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	_, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 7})
	require.NoError(t, err)

	var row InventoryTransaction
	require.NoError(t, db.Where("product_id = ?", p.ID).First(&row).Error)
	assert.Equal(t, "IN", row.Type)
	assert.Equal(t, 7, row.Quantity)
}
