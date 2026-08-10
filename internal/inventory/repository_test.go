package inventory

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"inventory/internal/category"
	"inventory/internal/product"
	sharederr "inventory/internal/shared/errors"
	"inventory/internal/warehouses"
)

// setupForRepo migrates the model set and returns a repository over a clean DB.
// It seeds the DEFAULT warehouse so stock movements without an explicit
// warehouse resolve to it, mirroring the production seed.
func setupForRepo(t *testing.T) (*gorm.DB, *GORMRepository, product.Product) {
	t.Helper()
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&warehouses.Warehouse{}, &category.Category{}, &product.Product{},
		&Inventory{}, &InventoryTransaction{},
	))
	require.NoError(t, db.Create(&warehouses.Warehouse{Code: "DEFAULT", Name: "Default Warehouse"}).Error)

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

	inv, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 15})
	require.NoError(t, err)
	assert.Equal(t, 15, inv.Quantity)
	assert.Equal(t, int64(1), countTx(t, db, p.ID))
}

func TestStockInThenStockOutNetsCorrectQty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	inv1, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 20})
	require.NoError(t, err)
	assert.Equal(t, 20, inv1.Quantity)

	inv2, err := repo.StockOut(context.Background(), Movement{ProductID: p.ID, Type: "OUT", Quantity: 6})
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
	_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 5})
	require.NoError(t, err)

	_, err = repo.StockOut(context.Background(), Movement{ProductID: p.ID, Type: "OUT", Quantity: 100})
	assert.ErrorIs(t, err, sharederr.ErrConflict)

	var qty int64
	require.NoError(t, db.Model(&Inventory{}).Where("product_id = ?", p.ID).Pluck("quantity", &qty).Error)
	assert.Equal(t, int64(5), qty, "quantity unchanged after overdraw")

	assert.Equal(t, int64(1), countTx(t, db, p.ID), "no partial OUT history row")
}

func TestStockInDeletedProductNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)

	require.NoError(t, repo.db.Delete(&product.Product{}, p.ID).Error)
	_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 1})
	assert.ErrorIs(t, err, sharederr.ErrNotFound, "stock-in on deleted product -> not found, not 500")
}

func TestStockOutNoRowConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)

	_, err := repo.StockOut(context.Background(), Movement{ProductID: p.ID, Type: "OUT", Quantity: 1})
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestStockInHistoryHasTypeAndQty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 7})
	require.NoError(t, err)

	var row InventoryTransaction
	require.NoError(t, db.Where("product_id = ?", p.ID).First(&row).Error)
	assert.Equal(t, "IN", row.Type)
	assert.Equal(t, 7, row.Quantity)
}

func TestListReturnsProductsWithZeroStock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, _ := setupForRepo(t)

	views, total, err := repo.List(context.Background(), ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, views, 1)
	assert.Equal(t, "WID-1", views[0].ProductSKU)
	assert.Equal(t, 0, views[0].Quantity, "no inventory row -> zero stock")
	assert.NotEmpty(t, views[0].UpdatedAt)
}

func TestListReflectsStockedQuantity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 25})
	require.NoError(t, err)

	views, total, err := repo.List(context.Background(), ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 25, views[0].Quantity)
}

func TestListFiltersByProductAndSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)

	byID, _, err := repo.List(context.Background(), ListQuery{ProductID: p.ID})
	require.NoError(t, err)
	assert.Len(t, byID, 1)

	other, _, err := repo.List(context.Background(), ListQuery{ProductID: uuid.New()})
	require.NoError(t, err)
	assert.Empty(t, other)

	bySearch, _, err := repo.List(context.Background(), ListQuery{Search: "wid"})
	require.NoError(t, err)
	assert.Len(t, bySearch, 1)

	noHit, _, err := repo.List(context.Background(), ListQuery{Search: "zzz-nomatch"})
	require.NoError(t, err)
	assert.Empty(t, noHit)
}

func TestListLowStockFilterMatchesThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 3})
	require.NoError(t, err)

	views, total, err := repo.List(context.Background(), ListQuery{LowStock: true})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 3, views[0].Quantity)
}

func TestListPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&category.Category{}, &product.Product{}, &Inventory{}, &InventoryTransaction{}))
	cat := category.Category{Name: "Various"}
	require.NoError(t, db.Create(&cat).Error)
	for i := 0; i < 3; i++ {
		p := product.Product{
			Name:  "Prod " + string(rune('A'+i)),
			SKU:   "SKU-" + string(rune('A'+i)),
			Price: 1, CategoryID: cat.ID,
		}
		require.NoError(t, db.Create(&p).Error)
	}
	repo := NewGORMRepository(db)

	page1, total, err := repo.List(context.Background(), ListQuery{Page: 1, PerPage: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, page1, 2)
}

func TestTransactionsReturnsJoinedHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 10, UserID: &p.ID})
	require.NoError(t, err)
	_, err = repo.StockOut(context.Background(), Movement{ProductID: p.ID, Type: "OUT", Quantity: 4, UserID: &p.ID})
	require.NoError(t, err)

	views, total, err := repo.Transactions(context.Background(), TransactionQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, views, 2)
	assert.Equal(t, "WID-1", views[0].ProductSKU)
	assert.NotEmpty(t, views[0].ProductName)
	assert.NotEmpty(t, views[0].CreatedAt)
}

func TestTransactionsFilterByTypeAndProduct(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 10})
	require.NoError(t, err)
	_, err = repo.StockOut(context.Background(), Movement{ProductID: p.ID, Type: "OUT", Quantity: 4})
	require.NoError(t, err)

	onlyOut, total, err := repo.Transactions(context.Background(), TransactionQuery{Type: "OUT"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "OUT", onlyOut[0].Type)

	byProduct, _, err := repo.Transactions(context.Background(), TransactionQuery{ProductID: p.ID})
	require.NoError(t, err)
	assert.Len(t, byProduct, 2)

	miss, total2, err := repo.Transactions(context.Background(), TransactionQuery{ProductID: uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total2)
	assert.Empty(t, miss)
}

func TestTransactionsRejectsInvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, _ := setupForRepo(t)

	_, _, err := repo.Transactions(context.Background(), TransactionQuery{Type: "SIDE"})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestTransactionsPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	for i := 0; i < 3; i++ {
		_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 1})
		require.NoError(t, err)
	}

	page, total, err := repo.Transactions(context.Background(), TransactionQuery{Page: 1, PerPage: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, page, 2)
}

func TestTransactionsPaginationZeroPerPageClampsToDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	for i := 0; i < 3; i++ {
		_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 1})
		require.NoError(t, err)
	}

	page, total, err := repo.Transactions(context.Background(), TransactionQuery{Page: 1, PerPage: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, page, 3, "should default to per_page 20")
}

// setupTransferRepo prepares a product with stock in the DEFAULT warehouse
// and a second warehouse.
func setupTransferRepo(t *testing.T) (*gorm.DB, *GORMRepository, product.Product, uuid.UUID, uuid.UUID) {
	t.Helper()
	db, repo, p := setupForRepo(t)

	whDefault, err := repo.DefaultWarehouse(context.Background())
	require.NoError(t, err)

	wh2 := warehouses.Warehouse{Code: "WH2", Name: "Warehouse 2"}
	require.NoError(t, db.Create(&wh2).Error)

	_, err = repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 20, WarehouseID: &whDefault})
	require.NoError(t, err)

	return db, repo, p, whDefault, wh2.ID
}

func TestTransferMovesStockAtomically(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p, from, to := setupTransferRepo(t)

	dst, err := repo.Transfer(context.Background(), Transfer{
		ProductID:       p.ID,
		FromWarehouseID: from,
		ToWarehouseID:   to,
		Quantity:        6,
	})
	require.NoError(t, err)
	assert.Equal(t, 6, dst.Quantity)

	var srcQty int64
	require.NoError(t, db.Model(&Inventory{}).
		Where("product_id = ? AND warehouse_id = ?", p.ID, from).Pluck("quantity", &srcQty).Error)
	assert.Equal(t, int64(14), srcQty)

	var outRow, inRow InventoryTransaction
	require.NoError(t, db.Where("product_id = ? AND type = ? AND warehouse_id = ?", p.ID, "OUT", from).First(&outRow).Error)
	require.NoError(t, db.Where("product_id = ? AND type = ? AND warehouse_id = ?", p.ID, "IN", to).First(&inRow).Error)
	require.NotNil(t, outRow.TransferID)
	require.NotNil(t, inRow.TransferID)
	assert.Equal(t, *outRow.TransferID, *inRow.TransferID)
}

func TestTransferOverdrawRollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p, from, to := setupTransferRepo(t)

	_, err := repo.Transfer(context.Background(), Transfer{
		ProductID:       p.ID,
		FromWarehouseID: from,
		ToWarehouseID:   to,
		Quantity:        999,
	})
	assert.ErrorIs(t, err, sharederr.ErrConflict)

	var srcQty int64
	require.NoError(t, db.Model(&Inventory{}).
		Where("product_id = ? AND warehouse_id = ?", p.ID, from).Pluck("quantity", &srcQty).Error)
	assert.Equal(t, int64(20), srcQty)

	var dstCount int64
	require.NoError(t, db.Model(&Inventory{}).
		Where("product_id = ? AND warehouse_id = ?", p.ID, to).Count(&dstCount).Error)
	assert.Equal(t, int64(0), dstCount)

	var txCount int64
	require.NoError(t, db.Model(&InventoryTransaction{}).
		Where("product_id = ? AND transfer_id IS NOT NULL", p.ID).Count(&txCount).Error)
	assert.Equal(t, int64(0), txCount, "no partial transfer history rows")
}

func TestTransferUnknownWarehouseNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p, from, _ := setupTransferRepo(t)

	_, err := repo.Transfer(context.Background(), Transfer{
		ProductID:       p.ID,
		FromWarehouseID: from,
		ToWarehouseID:   uuid.New(),
		Quantity:        1,
	})
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestTransferStockIsolatedPerWarehouse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p, whDefault, wh2 := setupTransferRepo(t)

	_, err := repo.StockOut(context.Background(), Movement{ProductID: p.ID, Type: "OUT", Quantity: 5, WarehouseID: &wh2})
	assert.ErrorIs(t, err, sharederr.ErrConflict)

	_, err = repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 7, WarehouseID: &wh2})
	require.NoError(t, err)

	var defaultQty, wh2Qty int64
	require.NoError(t, db.Model(&Inventory{}).Where("product_id = ? AND warehouse_id = ?", p.ID, whDefault).Pluck("quantity", &defaultQty).Error)
	require.NoError(t, db.Model(&Inventory{}).Where("product_id = ? AND warehouse_id = ?", p.ID, wh2).Pluck("quantity", &wh2Qty).Error)
	assert.Equal(t, int64(20), defaultQty)
	assert.Equal(t, int64(7), wh2Qty)
}

func TestListAggregatesAcrossWarehouses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p, _, wh2 := setupTransferRepo(t)
	_, err := repo.StockIn(context.Background(), Movement{ProductID: p.ID, Type: "IN", Quantity: 30, WarehouseID: &wh2})
	require.NoError(t, err)

	views, total, err := repo.List(context.Background(), ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 50, views[0].Quantity)

	views, total, err = repo.List(context.Background(), ListQuery{WarehouseID: &wh2})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 30, views[0].Quantity)
	_ = db
}

func TestTransactionsFilterByWarehouseAndTransferID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p, from, to := setupTransferRepo(t)
	_, err := repo.Transfer(context.Background(), Transfer{
		ProductID:       p.ID,
		FromWarehouseID: from,
		ToWarehouseID:   to,
		Quantity:        4,
	})
	require.NoError(t, err)

	views, total, err := repo.Transactions(context.Background(), TransactionQuery{WarehouseID: &to})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "IN", views[0].Type)
	require.NotNil(t, views[0].WarehouseID)
	assert.Equal(t, to, *views[0].WarehouseID)
	require.NotNil(t, views[0].TransferID)

	views2, _, err := repo.Transactions(context.Background(), TransactionQuery{WarehouseID: &from})
	require.NoError(t, err)
	assert.Equal(t, "OUT", views2[0].Type)
	require.NotNil(t, views2[0].TransferID)
	assert.Equal(t, *views[0].TransferID, *views2[0].TransferID)
	_ = db
}
