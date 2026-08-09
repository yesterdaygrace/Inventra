package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := "host=localhost user=postgres password=postgres dbname=inventory port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	// Clean up after test
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS activity_logs CASCADE")
		db.Exec("DROP TABLE IF EXISTS refresh_tokens CASCADE")
		db.Exec("DROP TABLE IF EXISTS users CASCADE")
		db.Exec("DROP TABLE IF EXISTS roles CASCADE")
	})

	return db
}

func TestRefreshToken_AutoMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)

	// Migrate using real models from auth package
	err := db.AutoMigrate(&Role{}, &User{}, &RefreshToken{})

	// Then
	require.NoError(t, err)

	// Verify table exists
	var tableExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables " +
		"WHERE table_schema = 'public' AND table_name = 'refresh_tokens')").
		Scan(&tableExists).Error
	require.NoError(t, err)
	assert.True(t, tableExists, "refresh_tokens table should exist")

	// Verify unique index on token_hash
	var indexExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE tablename = 'refresh_tokens' 
			AND indexdef LIKE '%token_hash%' 
			AND indexdef LIKE '%UNIQUE%'
		)
	`).Scan(&indexExists).Error
	require.NoError(t, err)
	assert.True(t, indexExists, "unique index on token_hash should exist")
}

func TestRefreshToken_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)

	err := db.AutoMigrate(&Role{}, &User{}, &RefreshToken{})
	require.NoError(t, err)

	// Create a test user
	role := Role{Name: "STAFF"}
	err = db.Create(&role).Error
	require.NoError(t, err)

	user := User{
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		RoleID:       role.ID,
	}
	err = db.Create(&user).Error
	require.NoError(t, err)

	// When
	token := RefreshToken{
		UserID:    user.ID,
		TokenHash: "test_hash_unique_123",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	err = db.Create(&token).Error

	// Then
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, token.ID)
	assert.Equal(t, user.ID, token.UserID)
	assert.Equal(t, "test_hash_unique_123", token.TokenHash)
	assert.False(t, token.CreatedAt.IsZero())
}

func TestRefreshToken_UniqueTokenHash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)

	err := db.AutoMigrate(&Role{}, &User{}, &RefreshToken{})
	require.NoError(t, err)

	role := Role{Name: "STAFF"}
	err = db.Create(&role).Error
	require.NoError(t, err)

	user := User{
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		RoleID:       role.ID,
	}
	err = db.Create(&user).Error
	require.NoError(t, err)

	token1 := RefreshToken{
		UserID:    user.ID,
		TokenHash: "duplicate_hash",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	err = db.Create(&token1).Error
	require.NoError(t, err)

	// When: try to create another token with same hash
	token2 := RefreshToken{
		UserID:    user.ID,
		TokenHash: "duplicate_hash",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	err = db.Create(&token2).Error

	// Then: should fail due to unique constraint
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestRefreshToken_RevokedAt_Nullable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupTestDB(t)

	err := db.AutoMigrate(&Role{}, &User{}, &RefreshToken{})
	require.NoError(t, err)

	role := Role{Name: "STAFF"}
	err = db.Create(&role).Error
	require.NoError(t, err)

	user := User{
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		RoleID:       role.ID,
	}
	err = db.Create(&user).Error
	require.NoError(t, err)

	// When: create token without RevokedAt
	token := RefreshToken{
		UserID:    user.ID,
		TokenHash: "test_hash_nullable",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	err = db.Create(&token).Error
	require.NoError(t, err)

	// Then: RevokedAt should be nil
	var retrieved RefreshToken
	err = db.First(&retrieved, token.ID).Error
	require.NoError(t, err)
	assert.Nil(t, retrieved.RevokedAt)

	// When: update RevokedAt
	now := time.Now()
	retrieved.RevokedAt = &now
	err = db.Save(&retrieved).Error
	require.NoError(t, err)

	// Then: RevokedAt should be set
	var updated RefreshToken
	err = db.First(&updated, token.ID).Error
	require.NoError(t, err)
	require.NotNil(t, updated.RevokedAt)
	assert.WithinDuration(t, now, *updated.RevokedAt, time.Second)
}
