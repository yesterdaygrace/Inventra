package inventory

import (
	"time"
	"context"
	"errors"
	"sync"
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

	// Defensive pre-clean: filtered -run invocations skip other tests'
	// cleanups, so leftovers from a previous run may still be present.
	db.Exec("DROP TABLE IF EXISTS inventory_reservations CASCADE")
	db.Exec("DROP TABLE IF EXISTS inventory_ledger CASCADE")
	db.Exec("DROP TABLE IF EXISTS inventory CASCADE")
	db.Exec("DROP TABLE IF EXISTS products CASCADE")
	db.Exec("DROP TABLE IF EXISTS categories CASCADE")
	db.Exec("DROP TABLE IF EXISTS warehouses CASCADE")

	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS inventory_reservations CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory_ledger CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory CASCADE")
		db.Exec("DROP TABLE IF EXISTS products CASCADE")
		db.Exec("DROP TABLE IF EXISTS categories CASCADE")
		db.Exec("DROP TABLE IF EXISTS warehouses CASCADE")
	})

	require.NoError(t, db.AutoMigrate(
		&warehouses.Warehouse{}, &category.Category{}, &product.Product{},
		&Inventory{}, &LedgerEntry{}, &Reservation{},
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
	require.NoError(t, db.Model(&LedgerEntry{}).Where("product_id = ?", productID).Count(&c).Error)
	return c
}

func TestStockInCreatesRowAndHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	inv, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 15})
	require.NoError(t, err)
	assert.Equal(t, 15, inv.Quantity)
	assert.Equal(t, int64(1), countTx(t, db, p.ID))
}

func TestStockInThenStockOutNetsCorrectQty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	inv1, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 20})
	require.NoError(t, err)
	assert.Equal(t, 20, inv1.Quantity)

	inv2, err := repo.Issue(context.Background(), Movement{ProductID: p.ID, Type: LedgerIssue, Quantity: 6})
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
	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 5})
	require.NoError(t, err)

	_, err = repo.Issue(context.Background(), Movement{ProductID: p.ID, Type: LedgerIssue, Quantity: 100})
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
	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 1})
	assert.ErrorIs(t, err, sharederr.ErrNotFound, "stock-in on deleted product -> not found, not 500")
}

func TestStockOutNoRowConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)

	_, err := repo.Issue(context.Background(), Movement{ProductID: p.ID, Type: LedgerIssue, Quantity: 1})
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestStockInHistoryHasTypeAndQty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 7})
	require.NoError(t, err)

	var row LedgerEntry
	require.NoError(t, db.Where("product_id = ?", p.ID).First(&row).Error)
	assert.Equal(t, LedgerReceive, row.TransactionType)
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
	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 25})
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
	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 3})
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
	require.NoError(t, db.AutoMigrate(&category.Category{}, &product.Product{}, &Inventory{}, &LedgerEntry{}))
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
	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 10, UserID: &p.ID})
	require.NoError(t, err)
	_, err = repo.Issue(context.Background(), Movement{ProductID: p.ID, Type: LedgerIssue, Quantity: 4, UserID: &p.ID})
	require.NoError(t, err)

	views, total, err := repo.Ledger(context.Background(), LedgerQuery{})
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
	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 10})
	require.NoError(t, err)
	_, err = repo.Issue(context.Background(), Movement{ProductID: p.ID, Type: LedgerIssue, Quantity: 4})
	require.NoError(t, err)

	onlyOut, total, err := repo.Ledger(context.Background(), LedgerQuery{Type: LedgerIssue})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, LedgerIssue, onlyOut[0].TransactionType)

	byProduct, _, err := repo.Ledger(context.Background(), LedgerQuery{ProductID: p.ID})
	require.NoError(t, err)
	assert.Len(t, byProduct, 2)

	miss, total2, err := repo.Ledger(context.Background(), LedgerQuery{ProductID: uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total2)
	assert.Empty(t, miss)
}

func TestTransactionsRejectsInvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, _ := setupForRepo(t)

	_, _, err := repo.Ledger(context.Background(), LedgerQuery{Type: "SIDE"})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestTransactionsPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	for i := 0; i < 3; i++ {
		_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 1})
		require.NoError(t, err)
	}

	page, total, err := repo.Ledger(context.Background(), LedgerQuery{Page: 1, PerPage: 2})
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
		_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 1})
		require.NoError(t, err)
	}

	page, total, err := repo.Ledger(context.Background(), LedgerQuery{Page: 1, PerPage: 0})
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

	_, err = repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 20, WarehouseID: &whDefault})
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

	var outRow, inRow LedgerEntry
	require.NoError(t, db.Where("product_id = ? AND transaction_type = ? AND warehouse_id = ?", p.ID, LedgerTransferOut, from).First(&outRow).Error)
	require.NoError(t, db.Where("product_id = ? AND transaction_type = ? AND warehouse_id = ?", p.ID, LedgerTransferIn, to).First(&inRow).Error)
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
	require.NoError(t, db.Model(&LedgerEntry{}).
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

	_, err := repo.Issue(context.Background(), Movement{ProductID: p.ID, Type: LedgerIssue, Quantity: 5, WarehouseID: &wh2})
	assert.ErrorIs(t, err, sharederr.ErrConflict)

	_, err = repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 7, WarehouseID: &wh2})
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
	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 30, WarehouseID: &wh2})
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

	views, total, err := repo.Ledger(context.Background(), LedgerQuery{WarehouseID: &to})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, LedgerTransferIn, views[0].TransactionType)
	require.NotNil(t, views[0].WarehouseID)
	assert.Equal(t, to, views[0].WarehouseID)
	require.NotNil(t, views[0].TransferID)

	views2, _, err := repo.Ledger(context.Background(), LedgerQuery{WarehouseID: &from})
	require.NoError(t, err)
	assert.Equal(t, LedgerTransferOut, views2[0].TransactionType)
	require.NotNil(t, views2[0].TransferID)
	assert.Equal(t, *views[0].TransferID, *views2[0].TransferID)
	_ = db
}

func TestStockInIncrementsVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	inv, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, inv.Version, "first stock-in sets version to 1")

	inv, err = repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 5})
	require.NoError(t, err)
	assert.Equal(t, 2, inv.Version, "second stock-in bumps version to 2")

	inv, err = repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 3})
	require.NoError(t, err)
	assert.Equal(t, 3, inv.Version, "third stock-in bumps version to 3")
	assert.Equal(t, 18, inv.Quantity)

	var row Inventory
	require.NoError(t, db.Where("product_id = ?", p.ID).First(&row).Error)
	assert.Equal(t, 3, row.Version)
	assert.Equal(t, 18, row.Quantity)
	assert.Equal(t, 0, row.ReservedQuantity)
}

func TestStockOutIncrementsVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)

	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 20})
	require.NoError(t, err)

	inv, err := repo.Issue(context.Background(), Movement{ProductID: p.ID, Type: LedgerIssue, Quantity: 5})
	require.NoError(t, err)
	assert.Equal(t, 2, inv.Version, "stock-in(1) + stock-out(1) = version 2")
	assert.Equal(t, 15, inv.Quantity)
}

func TestTransferIncrementsVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p, from, to := setupTransferRepo(t)

	_, err := repo.Transfer(context.Background(), Transfer{
		ProductID:       p.ID,
		FromWarehouseID: from,
		ToWarehouseID:   to,
		Quantity:        6,
	})
	require.NoError(t, err)

	var src, dst Inventory
	require.NoError(t, db.Where("product_id = ? AND warehouse_id = ?", p.ID, from).First(&src).Error)
	require.NoError(t, db.Where("product_id = ? AND warehouse_id = ?", p.ID, to).First(&dst).Error)
	assert.Equal(t, 2, src.Version, "source version: 1 from seeding stock-in + 1 from transfer")
	assert.Equal(t, 1, dst.Version, "destination version set to 1")
	assert.Equal(t, 14, src.Quantity)
	assert.Equal(t, 6, dst.Quantity)
}

func TestTransactionPersistsReferenceFields(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)

	refType := "PURCHASE_ORDER"
	refID := "PO-00123"
	reason := "Quarterly restock"

	_, err := repo.Receive(context.Background(), Movement{
		ProductID:     p.ID,
		Type:          LedgerReceive,
		Quantity:      10,
		ReferenceType: &refType,
		ReferenceID:   &refID,
		Reason:        &reason,
	})
	require.NoError(t, err)

	views, total, err := repo.Ledger(context.Background(), LedgerQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, views, 1)
	require.NotNil(t, views[0].ReferenceType)
	assert.Equal(t, "PURCHASE_ORDER", *views[0].ReferenceType)
	require.NotNil(t, views[0].ReferenceID)
	assert.Equal(t, "PO-00123", *views[0].ReferenceID)
	require.NotNil(t, views[0].Reason)
	assert.Equal(t, "Quarterly restock", *views[0].Reason)
}

func TestTransactionReferenceFieldsNullable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)

	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 5})
	require.NoError(t, err)

	views, _, err := repo.Ledger(context.Background(), LedgerQuery{})
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Nil(t, views[0].ReferenceType)
	assert.Nil(t, views[0].ReferenceID)
	assert.Nil(t, views[0].Reason)
}

func TestReservedQuantityRejectsNegative(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)

	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: 10})
	require.NoError(t, err)

	err = db.Model(&Inventory{}).Where("product_id = ?", p.ID).Update("reserved_quantity", -1).Error
	assert.Error(t, err, "CHECK violation on reserved_quantity = -1")
}

// seedQuantity stocks the product into the DEFAULT warehouse with n units.
func seedQuantity(t *testing.T, repo *GORMRepository, p product.Product, n int) {
	t.Helper()
	_, err := repo.Receive(context.Background(), Movement{ProductID: p.ID, Type: LedgerReceive, Quantity: n})
	require.NoError(t, err)
}

// TestConcurrentStockOutLeavesNoNegative is C7: with stock=100 and two
// concurrent draws of 70 and 50 on the same row, exactly one must succeed
// (the other fails ErrConflict) and the final quantity must be exactly 30 —
// never negative, never lost-update.
func TestConcurrentStockOutLeavesNoNegative(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)
	seedQuantity(t, repo, p, 100)

	const draws = 20
	for i := 0; i < draws; i++ {
		db.Exec("DELETE FROM inventory_ledger")
		// reseed to a clean 100 for this iteration
		require.NoError(t, db.Model(&Inventory{}).Where("product_id = ?", p.ID).Update("quantity", 100).Error)

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make(chan error, 2)
		results := make(chan *Inventory, 2)
		for _, qty := range []int{70, 50} {
			wg.Add(1)
			go func(q int) {
				defer wg.Done()
				<-start
				inv, err := repo.Issue(context.Background(), Movement{
					ProductID: p.ID, Type: LedgerIssue, Quantity: q,
				})
				errs <- err
				results <- inv
			}(qty)
		}
		close(start)
		wg.Wait()
		close(errs)
		close(results)

		var successes, failures int
		for err := range errs {
			if err == nil {
				successes++
			} else if errors.Is(err, sharederr.ErrConflict) {
				failures++
			} else {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		assert.Equal(t, 1, successes, "iteration %d: exactly one draw succeeds", i)
		assert.Equal(t, 1, failures, "iteration %d: exactly one draw conflicts", i)

		var qty int64
		require.NoError(t, db.Model(&Inventory{}).Where("product_id = ?", p.ID).Pluck("quantity", &qty).Error)
		assert.Contains(t, []int64{30, 50}, qty, "iteration %d: final quantity must be 30 (70-won) or 50 (50-won)", i)

		var txRows int64
		require.NoError(t, db.Model(&LedgerEntry{}).Where("product_id = ?", p.ID).Count(&txRows).Error)
		assert.Equal(t, int64(1), txRows, "iteration %d: exactly one OUT row from the winner", i)
	}
}

// TestConcurrentTransfersKeepSumsConsistent is C7: W1=100, W2=0, two
// concurrent transfers of 70 and 50 W1→W2. One wins, one conflicts, the
// total across warehouses stays 100, and exactly one transfer (two rows,
// one transfer_id) is recorded.
func TestConcurrentTransfersKeepSumsConsistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p, from, to := setupTransferRepo(t)
	require.NoError(t, db.Model(&Inventory{}).Where("product_id = ? AND warehouse_id = ?", p.ID, to).Delete(&Inventory{}).Error)

	const draws = 20
	for i := 0; i < draws; i++ {
		db.Exec("DELETE FROM inventory_ledger")
		require.NoError(t, db.Model(&Inventory{}).
			Where("product_id = ? AND warehouse_id = ?", p.ID, from).
			Update("quantity", 100).Error)
		require.NoError(t, db.Model(&Inventory{}).
			Where("product_id = ? AND warehouse_id = ?", p.ID, to).
			Update("quantity", 0).Error)

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make(chan error, 2)
		for _, qty := range []int{70, 50} {
			wg.Add(1)
			go func(q int) {
				defer wg.Done()
				<-start
				_, err := repo.Transfer(context.Background(), Transfer{
					ProductID:       p.ID,
					FromWarehouseID: from,
					ToWarehouseID:   to,
					Quantity:        q,
				})
				errs <- err
			}(qty)
		}
		close(start)
		wg.Wait()
		close(errs)

		var successes, failures int
		for err := range errs {
			if err == nil {
				successes++
			} else if errors.Is(err, sharederr.ErrConflict) {
				failures++
			} else {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		assert.Equal(t, 1, successes, "iteration %d: exactly one transfer succeeds", i)
		assert.Equal(t, 1, failures, "iteration %d: exactly one transfer conflicts", i)

		var sum int64
		require.NoError(t, db.Model(&Inventory{}).Where("product_id = ?", p.ID).Select("COALESCE(SUM(quantity),0)").Scan(&sum).Error)
		assert.Equal(t, int64(100), sum, "iteration %d: total across warehouses conserved", i)

		var transfers int64
		require.NoError(t, db.Model(&LedgerEntry{}).
			Where("product_id = ? AND transfer_id IS NOT NULL", p.ID).
			Distinct("transfer_id").Count(&transfers).Error)
		assert.Equal(t, int64(1), transfers, "iteration %d: exactly one transfer record", i)

		var outRows int64
		require.NoError(t, db.Model(&LedgerEntry{}).
			Where("product_id = ? AND transaction_type = ? AND warehouse_id = ?", p.ID, LedgerTransferOut, from).
			Count(&outRows).Error)
		assert.Equal(t, int64(1), outRows, "iteration %d: exactly one OUT row", i)

		var inRows int64
		require.NoError(t, db.Model(&LedgerEntry{}).
			Where("product_id = ? AND transaction_type = ? AND warehouse_id = ?", p.ID, LedgerTransferIn, to).
			Count(&inRows).Error)
		assert.Equal(t, int64(1), inRows, "iteration %d: exactly one IN row", i)
	}
}

func TestReservationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)
	wh := warehouses.Warehouse{Code: "WH-RSV", Name: "Reservation WH"}
	require.NoError(t, db.Create(&wh).Error)

	// Receive 50 into the warehouse.
	_, err := repo.Receive(context.Background(), Movement{
		ProductID: p.ID, Type: LedgerReceive, Quantity: 50, WarehouseID: &wh.ID,
	})
	require.NoError(t, err)

	// Reserve 20 for an order.
	rsv, err := repo.CreateReservation(context.Background(), Reservation{
		ProductID: p.ID, WarehouseID: wh.ID, Quantity: 20,
		ReferenceType: "order", ReferenceID: "ord-1", Status: ReservationActive,
	})
	require.NoError(t, err)
	require.Equal(t, ReservationActive, rsv.Status)

	// Reserved quantity is now 20; plain issue of 40 must fail (only 30 available).
	_, err = repo.Issue(context.Background(), Movement{
		ProductID: p.ID, Type: LedgerIssue, Quantity: 40, WarehouseID: &wh.ID,
	})
	assert.ErrorIs(t, err, sharederr.ErrInsufficientStock)

	// Issue of 30 succeeds.
	inv, err := repo.Issue(context.Background(), Movement{
		ProductID: p.ID, Type: LedgerIssue, Quantity: 30, WarehouseID: &wh.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, 20, inv.Quantity)

	// Release returns the 20 reserved units to available stock.
	released, err := repo.ReleaseReservation(context.Background(), rsv.ID)
	require.NoError(t, err)
	assert.Equal(t, ReservationReleased, released.Status)

	var after Inventory
	require.NoError(t, db.Where("product_id = ? AND warehouse_id = ?", p.ID, wh.ID).First(&after).Error)
	assert.Equal(t, 0, after.ReservedQuantity)

	// Consume flow: reserve again, then consume turns it into an ISSUE.
	rsv2, err := repo.CreateReservation(context.Background(), Reservation{
		ProductID: p.ID, WarehouseID: wh.ID, Quantity: 10,
		ReferenceType: "order", ReferenceID: "ord-2", Status: ReservationActive,
	})
	require.NoError(t, err)

	consumed, invAfter, err := repo.ConsumeReservation(context.Background(), rsv2.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, ReservationConsumed, consumed.Status)
	assert.Equal(t, 10, invAfter.Quantity)
	assert.Equal(t, 0, invAfter.ReservedQuantity)

	// The consumption wrote an ISSUE ledger row referencing the reservation.
	var entry LedgerEntry
	require.NoError(t, db.Where("product_id = ? AND transaction_type = ?", p.ID, LedgerIssue).
		Order("created_at DESC").First(&entry).Error)
	assert.Equal(t, "reservation", *entry.ReferenceType)
	assert.Equal(t, rsv2.ID.String(), *entry.ReferenceID)

	// Double-consume conflicts.
	_, _, err = repo.ConsumeReservation(context.Background(), rsv2.ID, nil)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestReservationLazyExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo, p := setupForRepo(t)
	wh := warehouses.Warehouse{Code: "WH-RSV-EXP", Name: "Expiry WH"}
	require.NoError(t, db.Create(&wh).Error)

	_, err := repo.Receive(context.Background(), Movement{
		ProductID: p.ID, Type: LedgerReceive, Quantity: 25, WarehouseID: &wh.ID,
	})
	require.NoError(t, err)

	// Reserve 15 with an expiry already in the past.
	past := time.Now().Add(-time.Hour)
	rsv, err := repo.CreateReservation(context.Background(), Reservation{
		ProductID: p.ID, WarehouseID: wh.ID, Quantity: 15,
		ReferenceType: "order", ReferenceID: "ord-stale", Status: ReservationActive,
		ExpiresAt: &past,
	})
	require.NoError(t, err)

	// A fresh issue observes the expired reservation, releases it, and can
	// then use the full on-hand quantity.
	inv, err := repo.Issue(context.Background(), Movement{
		ProductID: p.ID, Type: LedgerIssue, Quantity: 20, WarehouseID: &wh.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, 5, inv.Quantity)
	assert.Equal(t, 0, inv.ReservedQuantity)

	var stale Reservation
	require.NoError(t, db.First(&stale, "id = ?", rsv.ID).Error)
	assert.Equal(t, ReservationExpired, stale.Status)
}
