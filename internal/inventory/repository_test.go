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

func TestListReturnsProductsWithZeroStock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, _ := setupForRepo(t)

	views, total, err := repo.List(ListQuery{})
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
	_, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 25})
	require.NoError(t, err)

	views, total, err := repo.List(ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 25, views[0].Quantity)
}

func TestListFiltersByProductAndSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)

	byID, _, err := repo.List(ListQuery{ProductID: p.ID})
	require.NoError(t, err)
	assert.Len(t, byID, 1)

	other, _, err := repo.List(ListQuery{ProductID: uuid.New()})
	require.NoError(t, err)
	assert.Empty(t, other)

	bySearch, _, err := repo.List(ListQuery{Search: "wid"})
	require.NoError(t, err)
	assert.Len(t, bySearch, 1)

	noHit, _, err := repo.List(ListQuery{Search: "zzz-nomatch"})
	require.NoError(t, err)
	assert.Empty(t, noHit)
}

func TestListLowStockFilterMatchesThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	_, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 3})
	require.NoError(t, err)

	views, total, err := repo.List(ListQuery{LowStock: true})
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
		p := product.Product{Name: "Prod " + string(rune('A'+i)), SKU: "SKU-" + string(rune('A'+i)), Price: 1, CategoryID: cat.ID}
		require.NoError(t, db.Create(&p).Error)
	}
	repo := NewGORMRepository(db)

	page1, total, err := repo.List(ListQuery{Page: 1, PerPage: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, page1, 2)
}

func TestTransactionsReturnsJoinedHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	_, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 10, UserID: &p.ID})
	require.NoError(t, err)
	_, err = repo.StockOut(Movement{ProductID: p.ID, Type: "OUT", Quantity: 4, UserID: &p.ID})
	require.NoError(t, err)

	views, total, err := repo.Transactions(TransactionQuery{})
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
	_, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 10})
	require.NoError(t, err)
	_, err = repo.StockOut(Movement{ProductID: p.ID, Type: "OUT", Quantity: 4})
	require.NoError(t, err)

	onlyOut, total, err := repo.Transactions(TransactionQuery{Type: "OUT"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "OUT", onlyOut[0].Type)

	byProduct, _, err := repo.Transactions(TransactionQuery{ProductID: p.ID})
	require.NoError(t, err)
	assert.Len(t, byProduct, 2)

	miss, total2, err := repo.Transactions(TransactionQuery{ProductID: uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total2)
	assert.Empty(t, miss)
}

func TestTransactionsRejectsInvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, _ := setupForRepo(t)

	_, _, err := repo.Transactions(TransactionQuery{Type: "SIDE"})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestTransactionsPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo, p := setupForRepo(t)
	for i := 0; i < 3; i++ {
		_, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 1})
		require.NoError(t, err)
	}

	page, total, err := repo.Transactions(TransactionQuery{Page: 1, PerPage: 2})
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
		_, err := repo.StockIn(Movement{ProductID: p.ID, Type: "IN", Quantity: 1})
		require.NoError(t, err)
	}

	page, total, err := repo.Transactions(TransactionQuery{Page: 1, PerPage: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, page, 3, "should default to per_page 20")
}
