package user

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"inventory/internal/auth"
	sharederr "inventory/internal/shared/errors"
)

func setupRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "host=localhost user=postgres password=postgres dbname=inventory port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS activity_logs CASCADE")
		db.Exec("DROP TABLE IF EXISTS refresh_tokens CASCADE")
		db.Exec("DROP TABLE IF EXISTS users CASCADE")
		db.Exec("DROP TABLE IF EXISTS roles CASCADE")
	})

	require.NoError(t, db.AutoMigrate(&auth.Role{}, &auth.User{}))
	return db
}

func createRole(t *testing.T, db *gorm.DB, name string) auth.Role {
	t.Helper()
	r := auth.Role{Name: name}
	require.NoError(t, db.Create(&r).Error)
	return r
}

func createUser(t *testing.T, db *gorm.DB, name, email string, roleID uuid.UUID, active bool) auth.User {
	t.Helper()
	u := auth.User{
		Name:         name,
		Email:        email,
		PasswordHash: "hashed",
		RoleID:       roleID,
		IsActive:     active,
	}
	require.NoError(t, db.Create(&u).Error)
	return u
}

func TestRepo_FindRoleByName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupRepoDB(t)
	admin := createRole(t, db, "ADMIN")
	repo := NewGORMRepository(db)

	got, err := repo.FindRoleByName("ADMIN")
	require.NoError(t, err)
	assert.Equal(t, admin.ID, got.ID)

	_, err = repo.FindRoleByName("NOPE")
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestRepo_FindByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupRepoDB(t)
	staff := createRole(t, db, "STAFF")
	u := createUser(t, db, "Alice", "alice@example.com", staff.ID, true)
	repo := NewGORMRepository(db)

	got, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alice", got.Name)
	assert.Equal(t, "STAFF", got.Role.Name)

	_, err = repo.FindByID(uuid.New())
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestRepo_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupRepoDB(t)
	staff := createRole(t, db, "STAFF")
	u := createUser(t, db, "Alice", "alice@example.com", staff.ID, true)
	repo := NewGORMRepository(db)

	u.Name = "Alice Updated"
	u.IsActive = false
	require.NoError(t, repo.Update(&u))

	got, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alice Updated", got.Name)
	assert.False(t, got.IsActive)
}

func TestRepo_ListSearchRolePagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupRepoDB(t)
	admin := createRole(t, db, "ADMIN")
	staff := createRole(t, db, "STAFF")
	createUser(t, db, "Alice Admin", "alice@example.com", admin.ID, true)
	createUser(t, db, "Bob Staff", "bob@example.com", staff.ID, true)
	createUser(t, db, "Carol Staff", "carol@example.com", staff.ID, true)
	repo := NewGORMRepository(db)

	// filter by name substring
	users, total, err := repo.List(Query{Name: "carol"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)
	assert.Equal(t, "Carol Staff", users[0].Name)

	// filter by email substring
	users, total, err = repo.List(Query{Email: "bob"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)
	assert.Equal(t, "Bob Staff", users[0].Name)

	// role filter
	users, total, err = repo.List(Query{Role: "STAFF"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, users, 2)

	// pagination: per_page 2 -> page 2 returns remaining
	users, total, err = repo.List(Query{Page: 2, PerPage: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, users, 1)

	// no matches
	users, total, err = repo.List(Query{Name: "zzz"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, users)
}

func TestRepo_CountAdmins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupRepoDB(t)
	admin := createRole(t, db, "ADMIN")
	staff := createRole(t, db, "STAFF")
	createUser(t, db, "Admin One", "a1@example.com", admin.ID, true)
	createUser(t, db, "Admin Two", "a2@example.com", admin.ID, true)
	createUser(t, db, "Staff One", "s1@example.com", staff.ID, true)
	repo := NewGORMRepository(db)

	count, err := repo.CountAdmins()
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
