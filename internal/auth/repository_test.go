package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

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
	require.NoError(t, repo.CreateUser(u))
	assert.NotEqual(t, uuid.Nil, u.ID)

	// duplicate email -> ErrEmailTaken
	dup := &User{Name: "Ada2", Email: "ada@create.test", PasswordHash: "hash", RoleID: role.ID}
	err := repo.CreateUser(dup)
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

	got, err := repo.FindUserByEmail(user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, user.Email, got.Email)

	_, err = repo.FindUserByEmail("missing@repo.test")
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

	got, err := repo.FindUserByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Name, got.Name)

	_, err = repo.FindUserByID(uuid.New())
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
	require.NoError(t, repo.UpdateUser(&user))

	got, err := repo.FindUserByID(user.ID)
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

	got, err := repo.FindRoleByName("ADMIN")
	require.NoError(t, err)
	assert.Equal(t, role.ID, got.ID)

	_, err = repo.FindRoleByName("NOPE")
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

	got, err := repo.FindRoleByID(role.ID)
	require.NoError(t, err)
	assert.Equal(t, "STAFF", got.Name)

	_, err = repo.FindRoleByID(uuid.New())
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
	require.NoError(t, repo.CreateRefreshToken(tok))

	got, err := repo.FindRefreshTokenByHash("hash-a")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.UserID)

	_, err = repo.FindRefreshTokenByHash("nope")
	assert.ErrorIs(t, err, sharederr.ErrNotFound)

	now := time.Now()
	got.RevokedAt = &now
	require.NoError(t, repo.UpdateRefreshToken(got))

	again, err := repo.FindRefreshTokenByHash("hash-a")
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
	err := repo.CreateActivityLog(ActivityLogEntry{
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
	assert.False(t, isUniqueViolation(nil))
	assert.False(t, isUniqueViolation(errors.New("boom")))
	assert.True(t, isUniqueViolation(errors.New(`ERROR: duplicate key value violates unique constraint "users_email_key"`)))
}
