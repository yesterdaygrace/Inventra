package category

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharederr "inventory/internal/shared/errors"
)

// testProductRow mirrors the physical products table so CountProductsFor can
// be exercised in-tests without importing the product package (which would
// create a category<->product import cycle).
type testProductRow struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CategoryID uuid.UUID `gorm:"type:uuid;not null"`
	IsArchived bool      `gorm:"default:false"`
}

func (testProductRow) TableName() string { return "products" }

func TestRepo_CreateGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	desc := "electronics"
	c := &Category{Name: "Electronics", Description: &desc}
	require.NoError(t, repo.Create(context.Background(), c))

	got, err := repo.Get(context.Background(), c.ID)
	require.NoError(t, err)
	assert.Equal(t, "Electronics", got.Name)
	assert.Equal(t, "electronics", *got.Description)
}

func TestRepo_CreateUniqueConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	require.NoError(t, repo.Create(context.Background(), &Category{Name: "Books"}))
	err := repo.Create(context.Background(), &Category{Name: "Books"})
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

func TestRepo_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	c := &Category{Name: "Old"}
	require.NoError(t, repo.Create(context.Background(), c))
	c.Name = "New"
	require.NoError(t, repo.Update(context.Background(), c))

	got, err := repo.Get(context.Background(), c.ID)
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
}

func TestRepo_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)

	c := &Category{Name: "Temp", IsActive: true}
	require.NoError(t, repo.Create(context.Background(), c))
	require.NoError(t, repo.Delete(context.Background(), c.ID))

	got, err := repo.Get(context.Background(), c.ID)
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
	require.NoError(t, db.AutoMigrate(append(testModels, &testProductRow{})...))
	repo := NewGORMRepository(db)

	require.NoError(t, repo.Create(context.Background(), &Category{Name: "Audio"}))
	require.NoError(t, repo.Create(context.Background(), &Category{Name: "Book"}))
	require.NoError(t, repo.Create(context.Background(), &Category{Name: "Electronics"}))

	// search
	cats, total, err := repo.List(context.Background(), ListQuery{Search: "book"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, cats, 1)
	assert.Equal(t, "Book", cats[0].Name)

	// name sort (default ASC)
	cats, _, err = repo.List(context.Background(), ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, "Audio", cats[0].Name)
	assert.Equal(t, "Electronics", cats[2].Name)

	// name DESC
	cats, _, err = repo.List(context.Background(), ListQuery{Sort: "-name"})
	require.NoError(t, err)
	assert.Equal(t, "Electronics", cats[0].Name)

	// pagination
	cats, total, err = repo.List(context.Background(), ListQuery{Page: 2, PerPage: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, cats, 1)
}

func TestRepo_CountProductsFor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(append(testModels, &testProductRow{})...))
	repo := NewGORMRepository(db)

	c := &Category{Name: "Refs"}
	require.NoError(t, repo.Create(context.Background(), c))

	count, err := repo.CountProductsFor(context.Background(), c.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	require.NoError(t, db.Create(&testProductRow{CategoryID: c.ID}).Error)
	require.NoError(t, db.Create(&testProductRow{CategoryID: c.ID}).Error)

	count, err = repo.CountProductsFor(context.Background(), c.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestGORMRepositoryImplementsInterface(t *testing.T) {
	var _ Repository = (*GORMRepository)(nil)
}
