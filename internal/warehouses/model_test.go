package warehouses

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var testModels = []any{
	&Warehouse{},
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "host=localhost user=postgres password=postgres dbname=inventory port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS inventory_transactions CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory CASCADE")
		db.Exec("DROP TABLE IF EXISTS products CASCADE")
		db.Exec("DROP TABLE IF EXISTS warehouses CASCADE")
	})

	return db
}

func TestWarehouse_AutoMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)
	require.NoError(t, err)

	var tableExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'warehouses')").Scan(&tableExists).Error
	require.NoError(t, err)
	assert.True(t, tableExists, "warehouses table should exist")
}

func TestWarehouse_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))

	desc := "Main warehouse"
	w := Warehouse{Code: "WH-001", Name: "Main", Description: &desc, IsActive: true}
	err := db.Create(&w).Error
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, w.ID)
	assert.False(t, w.CreatedAt.IsZero())
	assert.False(t, w.UpdatedAt.IsZero())
}

func TestWarehouse_UniqueCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))

	require.NoError(t, db.Create(&Warehouse{Code: "DUP", Name: "A"}).Error)
	err := db.Create(&Warehouse{Code: "DUP", Name: "B"}).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestWarehouse_TableName(t *testing.T) {
	assert.Equal(t, "warehouses", Warehouse{}.TableName())
}
