package auth

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupModelTestDB(t *testing.T) *gorm.DB {
	dsn := "host=localhost user=postgres password=postgres dbname=inventory port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS users CASCADE")
		db.Exec("DROP TABLE IF EXISTS roles CASCADE")
	})

	return db
}

func TestRoleUser_AutoMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupModelTestDB(t)

	// When
	err := db.AutoMigrate(&Role{}, &User{})

	// Then
	require.NoError(t, err)

	// Verify roles table exists
	var rolesExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'roles')").Scan(&rolesExists).Error
	require.NoError(t, err)
	assert.True(t, rolesExists, "roles table should exist")

	// Verify users table exists
	var usersExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'users')").Scan(&usersExists).Error
	require.NoError(t, err)
	assert.True(t, usersExists, "users table should exist")

	// Verify unique index on users.email
	var emailUniqueExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE tablename = 'users' 
			AND indexdef LIKE '%email%' 
			AND indexdef LIKE '%UNIQUE%'
		)
	`).Scan(&emailUniqueExists).Error
	require.NoError(t, err)
	assert.True(t, emailUniqueExists, "unique index on email should exist")
}

func TestRole_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupModelTestDB(t)
	err := db.AutoMigrate(&Role{}, &User{})
	require.NoError(t, err)

	// When
	role := Role{Name: "ADMIN"}
	err = db.Create(&role).Error

	// Then
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, role.ID)
	assert.Equal(t, "ADMIN", role.Name)
	assert.False(t, role.CreatedAt.IsZero())
}

func TestUser_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupModelTestDB(t)
	err := db.AutoMigrate(&Role{}, &User{})
	require.NoError(t, err)

	role := Role{Name: "STAFF"}
	err = db.Create(&role).Error
	require.NoError(t, err)

	// When
	user := User{
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		RoleID:       role.ID,
		IsActive:     true,
	}
	err = db.Create(&user).Error

	// Then
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "test@example.com", user.Email)
	assert.True(t, user.IsActive)
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
}

func TestUser_UniqueEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupModelTestDB(t)
	err := db.AutoMigrate(&Role{}, &User{})
	require.NoError(t, err)

	role := Role{Name: "STAFF"}
	err = db.Create(&role).Error
	require.NoError(t, err)

	user1 := User{
		Name:         "User One",
		Email:        "duplicate@example.com",
		PasswordHash: "hashed",
		RoleID:       role.ID,
	}
	err = db.Create(&user1).Error
	require.NoError(t, err)

	// When: try to create another user with same email
	user2 := User{
		Name:         "User Two",
		Email:        "duplicate@example.com",
		PasswordHash: "hashed",
		RoleID:       role.ID,
	}
	err = db.Create(&user2).Error

	// Then: should fail due to unique constraint
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestUser_RoleFK(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// Given
	db := setupModelTestDB(t)
	err := db.AutoMigrate(&Role{}, &User{})
	require.NoError(t, err)

	// When: try to create user with non-existent role_id
	user := User{
		Name:         "Orphan User",
		Email:        "orphan@example.com",
		PasswordHash: "hashed",
		RoleID:       uuid.New(), // Non-existent role
	}
	err = db.Create(&user).Error

	// Then: should fail due to FK constraint
	require.Error(t, err)
	assert.Contains(t, err.Error(), "violates foreign key constraint")
}
