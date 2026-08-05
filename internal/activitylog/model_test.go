package activitylog

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"inventory/internal/auth"
)

var testModels = []any{
	&auth.Role{},
	&auth.User{},
	&ActivityLog{},
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

func TestActivityLog_AutoMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)

	// Then
	require.NoError(t, err)

	// Verify activity_logs table exists
	var tableExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'activity_logs')").Scan(&tableExists).Error
	require.NoError(t, err)
	assert.True(t, tableExists, "activity_logs table should exist")
}

func TestActivityLog_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)
	require.NoError(t, err)

	role := auth.Role{Name: "ADMIN"}
	err = db.Create(&role).Error
	require.NoError(t, err)

	user := auth.User{
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		RoleID:       role.ID,
	}
	err = db.Create(&user).Error
	require.NoError(t, err)

	// When
	entityID := "test-entity-123"
	details := datatypes.JSON([]byte(`{"key": "value"}`))
	ip := "192.168.1.1"
	log := ActivityLog{
		UserID:     &user.ID,
		Action:     "CREATE",
		EntityType: "product",
		EntityID:   &entityID,
		Details:    &details,
		IP:         &ip,
	}
	err = db.Create(&log).Error

	// Then
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, log.ID)
	require.NotNil(t, log.UserID)
	assert.Equal(t, user.ID, *log.UserID)
	assert.Equal(t, "CREATE", log.Action)
	assert.Equal(t, "product", log.EntityType)
	require.NotNil(t, log.EntityID)
	assert.Equal(t, "test-entity-123", *log.EntityID)
	require.NotNil(t, log.IP)
	assert.Equal(t, "192.168.1.1", *log.IP)
	assert.False(t, log.CreatedAt.IsZero())
}

func TestActivityLog_CreateWithoutUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)
	err := db.AutoMigrate(testModels...)
	require.NoError(t, err)

	// When: create log without user (anonymous action)
	log := ActivityLog{
		UserID:     nil,
		Action:     "REGISTER",
		EntityType: "user",
	}
	err = db.Create(&log).Error

	// Then
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, log.ID)
	assert.Nil(t, log.UserID)
	assert.Equal(t, "REGISTER", log.Action)
	assert.Equal(t, "user", log.EntityType)
}

func TestActivityLog_TableName(t *testing.T) {
	// Given
	log := ActivityLog{}

	// When
	tableName := log.TableName()

	// Then
	assert.Equal(t, "activity_logs", tableName)
}
