package product

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"inventory/internal/category"
	sharederr "inventory/internal/shared/errors"
)

// setupForRepo migrates the category+product models and ensures the physical
// inventory table exists (created via raw SQL to avoid an import cycle).
func setupForRepo(t *testing.T) (*gorm.DB, *GORMRepository) {
	t.Helper()
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&category.Category{}, &Product{}))
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS inventory (
		id uuid primary key default gen_random_uuid(),
		product_id uuid unique not null,
		quantity int not null default 0,
		updated_at timestamptz
	)`).Error)
	return db, NewGORMRepository(db)
}

func mustCategory(t *testing.T, db *gorm.DB, name string) category.Category {
	t.Helper()
	cat := category.Category{Name: name}
	require.NoError(t, db.Create(&cat).Error)
	return cat
}

func mustProduct(t *testing.T, db *gorm.DB, p *Product) *Product {
	t.Helper()
	require.NoError(t, db.Create(p).Error)
	return p
}

func TestRepoCreateGetRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")

	p := mustProduct(t, db, &Product{Name: "Laptop", SKU: "LAP-001", Price: 999, CategoryID: cat.ID, LowStockThreshold: 5})

	got, err := repo.Get(context.Background(), p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Laptop", got.Name)
	assert.Equal(t, "LAP-001", got.SKU)
	assert.Equal(t, cat.ID, got.CategoryID)
	assert.Equal(t, "Electronics", got.Category.Name)
}

func TestRepoCreateUniqueSKUConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	mustProduct(t, db, &Product{Name: "A", SKU: "DUP", Price: 1, CategoryID: cat.ID})

	err := repo.Create(context.Background(), &Product{Name: "B", SKU: "DUP", Price: 2, CategoryID: cat.ID})
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestRepoSKUExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	wid := mustProduct(t, db, &Product{Name: "Widget", SKU: "wid", Price: 2, CategoryID: cat.ID})

	exists, err := repo.SKUExists(context.Background(), "WID", uuid.Nil)
	require.NoError(t, err)
	assert.True(t, exists, "case-insensitive SKU match expected")

	exists, err = repo.SKUExists(context.Background(), "WID", wid.ID)
	require.NoError(t, err)
	assert.False(t, exists, "excluded own row should not count")
}

func TestRepoUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	cat2 := mustCategory(t, db, "Books")
	p := mustProduct(t, db, &Product{Name: "Old", SKU: "UPD", Price: 10, CategoryID: cat.ID, LowStockThreshold: 5})

	p.Name = "New"
	p.SKU = "UPD"
	p.CategoryID = cat2.ID
	p.Price = 20
	require.NoError(t, repo.Update(context.Background(), p))

	got, err := repo.Get(context.Background(), p.ID)
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
	assert.Equal(t, "Books", got.Category.Name)
	assert.Equal(t, 20.0, got.Price)
}

func TestRepoDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	p := mustProduct(t, db, &Product{Name: "Gone", SKU: "DEL", Price: 1, CategoryID: cat.ID})

	require.NoError(t, repo.Delete(context.Background(), p.ID))

	got, err := repo.Get(context.Background(), p.ID)
	require.NoError(t, err)
	assert.True(t, got.IsArchived, "soft-archive should keep the row, flagged archived")

	assert.ErrorIs(t, repo.Delete(context.Background(), uuid.New()), sharederr.ErrNotFound)
}

func TestRepoGetNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	_, repo := setupForRepo(t)
	got, err := repo.Get(context.Background(), uuid.New())
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
	assert.Nil(t, got)
}

func TestRepoListSortEdges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	mustProduct(t, db, &Product{Name: "B", SKU: "SKU-B", Price: 1, CategoryID: cat.ID})
	mustProduct(t, db, &Product{Name: "A", SKU: "SKU-A", Price: 2, CategoryID: cat.ID})

	for _, sort := range []string{"name", "-name", "price", "-price", "created_at", "-created_at", "sku", "-sku", "garbage"} {
		prods, _, err := repo.List(context.Background(), ListQuery{Sort: sort, Page: 1, PerPage: 10})
		require.NoError(t, err, "sort %q", sort)
		assert.NotEmpty(t, prods, "sort %q should return rows", sort)
	}
}

func TestRepoListSearchAndSort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	mustProduct(t, db, &Product{Name: "Alpha", SKU: "SKU-A", Price: 30, CategoryID: cat.ID})
	mustProduct(t, db, &Product{Name: "Beta", SKU: "SKU-B", Price: 10, CategoryID: cat.ID})

	prods, total, err := repo.List(context.Background(), ListQuery{Q: "beta", Sort: "price", Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, prods, 1)
	assert.Equal(t, "Beta", prods[0].Name)

	prods, total, err = repo.List(context.Background(), ListQuery{Sort: "-price", Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, "Alpha", prods[0].Name)
}

func TestRepoListPriceRangeFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	mustProduct(t, db, &Product{Name: "Cheap", SKU: "C", Price: 5, CategoryID: cat.ID})
	mustProduct(t, db, &Product{Name: "Mid", SKU: "M", Price: 50, CategoryID: cat.ID})
	mustProduct(t, db, &Product{Name: "Pricey", SKU: "P", Price: 500, CategoryID: cat.ID})

	min, max := 10.0, 100.0
	prods, total, err := repo.List(context.Background(), ListQuery{MinPrice: &min, MaxPrice: &max, Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "Mid", prods[0].Name)
}

func TestRepoListCategoryFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	catA := mustCategory(t, db, "CatA")
	catB := mustCategory(t, db, "CatB")
	mustProduct(t, db, &Product{Name: "A1", SKU: "A1", Price: 1, CategoryID: catA.ID})
	mustProduct(t, db, &Product{Name: "B1", SKU: "B1", Price: 1, CategoryID: catB.ID})

	prods, total, err := repo.List(context.Background(), ListQuery{CategoryID: catA.ID, Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "A1", prods[0].Name)
}

func TestRepoListArchivedFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	mustProduct(t, db, &Product{Name: "Live", SKU: "L", Price: 1, CategoryID: cat.ID})
	mustProduct(t, db, &Product{Name: "Dead", SKU: "D", Price: 1, CategoryID: cat.ID, IsArchived: true})

	archived := true
	prods, total, err := repo.List(context.Background(), ListQuery{IsArchived: &archived, Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "Dead", prods[0].Name)
}

func TestRepoListLowStock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	low := mustProduct(t, db, &Product{Name: "Low", SKU: "LOW", Price: 1, CategoryID: cat.ID, LowStockThreshold: 10})
	high := mustProduct(t, db, &Product{Name: "High", SKU: "HIGH", Price: 1, CategoryID: cat.ID, LowStockThreshold: 10})

	require.NoError(t, db.Exec("INSERT INTO inventory (product_id, quantity) VALUES (?, 3)", low.ID).Error)
	require.NoError(t, db.Exec("INSERT INTO inventory (product_id, quantity) VALUES (?, 20)", high.ID).Error)

	prods, total, err := repo.List(context.Background(), ListQuery{LowStock: true, Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, prods, 1)
	assert.Equal(t, "Low", prods[0].Name)
}

func TestRepoListPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db, repo := setupForRepo(t)
	cat := mustCategory(t, db, "Electronics")
	for i := 0; i < 5; i++ {
		mustProduct(t, db, &Product{Name: "P", SKU: "S" + string(rune('A'+i)), Price: 1, CategoryID: cat.ID})
	}

	prods, total, err := repo.List(context.Background(), ListQuery{Page: 1, PerPage: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, prods, 2)
}
