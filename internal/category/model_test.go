package category

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var testModels = []any{
	&Category{},
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

func TestCategory_AutoMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)

	// When: AutoMigrate with testModels
	err := db.AutoMigrate(testModels...)

	// Then
	require.NoError(t, err)

	// Verify categories table exists
	var tableExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables " +
		"WHERE table_schema = 'public' AND table_name = 'categories')").
		Scan(&tableExists).Error
	require.NoError(t, err)
	assert.True(t, tableExists, "categories table should exist")

	// Verify unique index on name
	var uniqueExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE tablename = 'categories' 
			AND indexdef LIKE '%name%' 
			AND indexdef LIKE '%UNIQUE%'
		)
	`).Scan(&uniqueExists).Error
	require.NoError(t, err)
	assert.True(t, uniqueExists, "unique index on name should exist")
}

func TestCategory_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)
	require.NoError(t, err)

	// When
	desc := "Test description"
	category := Category{
		Name:        "Electronics",
		Description: &desc,
	}
	err = db.Create(&category).Error

	// Then
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, category.ID)
	assert.Equal(t, "Electronics", category.Name)
	require.NotNil(t, category.Description)
	assert.Equal(t, "Test description", *category.Description)
	assert.False(t, category.CreatedAt.IsZero())
	assert.False(t, category.UpdatedAt.IsZero())
}

func TestCategory_UniqueName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)
	require.NoError(t, err)

	category1 := Category{Name: "Furniture"}
	err = db.Create(&category1).Error
	require.NoError(t, err)

	// When: try to create another category with same name
	category2 := Category{Name: "Furniture"}
	err = db.Create(&category2).Error

	// Then: should fail due to unique constraint
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestCategory_TableName(t *testing.T) {
	// Given
	category := Category{}

	// When
	tableName := category.TableName()

	// Then
	assert.Equal(t, "categories", tableName)
}
