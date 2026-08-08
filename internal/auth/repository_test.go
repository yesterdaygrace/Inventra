package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"inventory/internal/shared/dbutil"

	sharederr "inventory/internal/shared/errors"
)

// seedRoleUser creates a role + user row and returns them.
func seedRoleUser(t *testing.T, db *gorm.DB, roleName string) (Role, User) {
	t.Helper()
	role := Role{Name: roleName}
	require.NoError(t, db.Create(&role).Error)
	user := User{Name: "Repo User", Email: roleName + "@repo.test", PasswordHash: "hash", RoleID: role.ID}
	require.NoError(t, db.Create(&user).Error)
	return role, user
}

func TestGORMRepository_ImplementsInterface(t *testing.T) {
	var _ Repository = (*GORMRepository)(nil)
}

func TestGORMRepository_CreateUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&Role{}, &User{}, &RefreshToken{}, &activityLogRow{}))
	repo := NewGORMRepository(db)

	role := Role{Name: "STAFF"}
	require.NoError(t, db.Create(&role).Error)

	u := &User{Name: "Ada", Email: "ada@create.test", PasswordHash: "hash", RoleID: role.ID}
	require.NoError(t, repo.CreateUser(context.Background(), u))
	assert.NotEqual(t, uuid.Nil, u.ID)

	// duplicate email -> ErrEmailTaken
	dup := &User{Name: "Ada2", Email: "ada@create.test", PasswordHash: "hash", RoleID: role.ID}
	err := repo.CreateUser(context.Background(), dup)
	assert.ErrorIs(t, err, ErrEmailTaken)
}

func TestGORMRepository_FindUserByEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&Role{}, &User{}, &RefreshToken{}, &activityLogRow{}))
	repo := NewGORMRepository(db)

	_, user := seedRoleUser(t, db, "STAFF")

	got, err := repo.FindUserByEmail(context.Background(), user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, user.Email, got.Email)

	_, err = repo.FindUserByEmail(context.Background(), "missing@repo.test")
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestGORMRepository_FindUserByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&Role{}, &User{}, &RefreshToken{}, &activityLogRow{}))
	repo := NewGORMRepository(db)

	_, user := seedRoleUser(t, db, "STAFF")

	got, err := repo.FindUserByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Name, got.Name)

	_, err = repo.FindUserByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestGORMRepository_UpdateUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&Role{}, &User{}, &RefreshToken{}, &activityLogRow{}))
	repo := NewGORMRepository(db)

	_, user := seedRoleUser(t, db, "STAFF")
	user.Name = "Renamed"
	require.NoError(t, repo.UpdateUser(context.Background(), &user))

	got, err := repo.FindUserByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Name)
}

func TestGORMRepository_FindRoleByName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&Role{}, &User{}, &RefreshToken{}, &activityLogRow{}))
	repo := NewGORMRepository(db)

	role, _ := seedRoleUser(t, db, "ADMIN")

	got, err := repo.FindRoleByName(context.Background(), "ADMIN")
	require.NoError(t, err)
	assert.Equal(t, role.ID, got.ID)

	_, err = repo.FindRoleByName(context.Background(), "NOPE")
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestGORMRepository_FindRoleByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&Role{}, &User{}, &RefreshToken{}, &activityLogRow{}))
	repo := NewGORMRepository(db)

	role, _ := seedRoleUser(t, db, "STAFF")

	got, err := repo.FindRoleByID(context.Background(), role.ID)
	require.NoError(t, err)
	assert.Equal(t, "STAFF", got.Name)

	_, err = repo.FindRoleByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestGORMRepository_RefreshTokenCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&Role{}, &User{}, &RefreshToken{}, &activityLogRow{}))
	repo := NewGORMRepository(db)

	_, user := seedRoleUser(t, db, "STAFF")

	tok := &RefreshToken{UserID: user.ID, TokenHash: "hash-a", ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, repo.CreateRefreshToken(context.Background(), tok))

	got, err := repo.FindRefreshTokenByHash(context.Background(), "hash-a")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.UserID)

	_, err = repo.FindRefreshTokenByHash(context.Background(), "nope")
	assert.ErrorIs(t, err, sharederr.ErrNotFound)

	now := time.Now()
	got.RevokedAt = &now
	require.NoError(t, repo.UpdateRefreshToken(context.Background(), got))

	again, err := repo.FindRefreshTokenByHash(context.Background(), "hash-a")
	require.NoError(t, err)
	require.NotNil(t, again.RevokedAt)
}

func TestGORMRepository_CreateActivityLog(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&Role{}, &User{}, &RefreshToken{}, &activityLogRow{}))
	repo := NewGORMRepository(db)

	uid := uuid.New()
	err := repo.CreateActivityLog(context.Background(), ActivityLogEntry{
		UserID:     &uid,
		Action:     "LOGIN",
		EntityType: "user",
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&activityLogRow{}).
		Where("user_id = ?", uid.String()).
		Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestIsUniqueViolation(t *testing.T) {
	assert.False(t, dbutil.IsUniqueViolation(nil))
	assert.False(t, dbutil.IsUniqueViolation(errors.New("boom")))
	assert.True(t, dbutil.IsUniqueViolation(errors.New(`ERROR: duplicate key value violates unique constraint "users_email_key"`)))
}
