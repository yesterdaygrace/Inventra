package warehouses

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharederr "inventory/internal/shared/errors"
)

// testInventoryRow mirrors the physical inventory table so CountInventoryFor
// can be exercised in-tests without importing the inventory package (which
// would create a warehouses<->inventory import cycle after T4 adds FKs).
type testInventoryRow struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WarehouseID uuid.UUID `gorm:"type:uuid;not null"`
}

func (testInventoryRow) TableName() string { return "inventory" }

func TestRepo_CreateGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	desc := "north side"
	w := &Warehouse{Code: "NORTH", Name: "North Warehouse", Description: &desc}
	require.NoError(t, repo.Create(context.Background(), w))

	got, err := repo.Get(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, "NORTH", got.Code)
	assert.Equal(t, "north side", *got.Description)
}

func TestRepo_GetByCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	require.NoError(t, repo.Create(context.Background(), &Warehouse{Code: "DEFAULT", Name: "Default"}))
	got, err := repo.GetByCode(context.Background(), "DEFAULT")
	require.NoError(t, err)
	assert.Equal(t, "Default", got.Name)

	_, err = repo.GetByCode(context.Background(), "NOPE")
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestRepo_CreateUniqueConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	require.NoError(t, repo.Create(context.Background(), &Warehouse{Code: "SAME", Name: "A"}))
	err := repo.Create(context.Background(), &Warehouse{Code: "SAME", Name: "B"})
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestRepo_GetNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	_, err := repo.Get(context.Background(), uuid.New())
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestRepo_UpdateCodeConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	require.NoError(t, repo.Create(context.Background(), &Warehouse{Code: "A", Name: "A"}))
	b := &Warehouse{Code: "B", Name: "B"}
	require.NoError(t, repo.Create(context.Background(), b))

	b.Code = "A"
	err := repo.Update(context.Background(), b)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestRepo_DeleteSoftDeactivates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	w := &Warehouse{Code: "TEMP", Name: "Temp", IsActive: true}
	require.NoError(t, repo.Create(context.Background(), w))
	require.NoError(t, repo.Delete(context.Background(), w.ID))

	got, err := repo.Get(context.Background(), w.ID)
	require.NoError(t, err)
	assert.False(t, got.IsActive, "soft-deactivate should keep the row, flagged inactive")

	err = repo.Delete(context.Background(), uuid.New())
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestRepo_ListSearchSortPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	require.NoError(t, repo.Create(context.Background(), &Warehouse{Code: "WH1", Name: "Alpha"}))
	require.NoError(t, repo.Create(context.Background(), &Warehouse{Code: "WH2", Name: "Beta"}))
	require.NoError(t, repo.Create(context.Background(), &Warehouse{Code: "WH3", Name: "Gamma"}))

	// search by name
	whs, total, err := repo.List(context.Background(), ListQuery{Search: "bet"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "Beta", whs[0].Name)

	// search by code
	whs, total, err = repo.List(context.Background(), ListQuery{Search: "wh2"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "WH2", whs[0].Code)

	// default sort name ASC
	whs, _, err = repo.List(context.Background(), ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, "Alpha", whs[0].Name)
	assert.Equal(t, "Gamma", whs[2].Name)

	// name DESC
	whs, _, err = repo.List(context.Background(), ListQuery{Sort: "-name"})
	require.NoError(t, err)
	assert.Equal(t, "Gamma", whs[0].Name)

	// pagination
	whs, total, err = repo.List(context.Background(), ListQuery{Page: 2, PerPage: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, whs, 1)
}

func TestRepo_ListIsActiveFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	require.NoError(t, repo.Create(context.Background(), &Warehouse{Code: "ON", Name: "On", IsActive: true}))
	req := &Warehouse{Code: "OFF", Name: "Off", IsActive: true}
	require.NoError(t, repo.Create(context.Background(), req))
	require.NoError(t, repo.Delete(context.Background(), req.ID)) // soft-deactivate OFF

	falseFilter := false
	whs, total, err := repo.List(context.Background(), ListQuery{IsActive: &falseFilter})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "Off", whs[0].Name)
}

func TestRepo_CountInventoryFor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(append(testModels, &testInventoryRow{})...))
	repo := NewGORMRepository(db)

	w := &Warehouse{Code: "STO", Name: "Storeroom"}
	require.NoError(t, repo.Create(context.Background(), w))

	count, err := repo.CountInventoryFor(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	require.NoError(t, db.Create(&testInventoryRow{WarehouseID: w.ID}).Error)
	require.NoError(t, db.Create(&testInventoryRow{WarehouseID: w.ID}).Error)

	count, err = repo.CountInventoryFor(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestRepo_ListWithInventoryCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(append(testModels, &testInventoryRow{})...))
	repo := NewGORMRepository(db)

	w := &Warehouse{Code: "CNT", Name: "Counted"}
	require.NoError(t, repo.Create(context.Background(), w))
	require.NoError(t, db.Create(&testInventoryRow{WarehouseID: w.ID}).Error)

	whs, total, err := repo.ListWithInventoryCount(context.Background(), ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, int64(1), whs[0].InventoryCount)
}

func TestGORMRepositoryImplementsInterface(t *testing.T) {
	var _ Repository = (*GORMRepository)(nil)
}
