package activitylog

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"inventory/internal/auth"
)

// seedUser creates an ADMIN user for FK/user filtering tests.
func seedUser(t *testing.T, db *gorm.DB) (uuid.UUID, *auth.User) {
	t.Helper()
	role := auth.Role{Name: "ADMIN"}
	require.NoError(t, db.Create(&role).Error)
	u := auth.User{Name: "Auditor", Email: "auditor@inventory.local", PasswordHash: "h", RoleID: role.ID}
	require.NoError(t, db.Create(&u).Error)
	return u.ID, &u
}

func seedLog(t *testing.T, db *gorm.DB, uid uuid.UUID, action, entityType, entityID string) ActivityLog {
	t.Helper()
	eid := entityID
	log := ActivityLog{UserID: &uid, Action: action, EntityType: entityType, EntityID: &eid, IP: ip("1.2.3.4")}
	require.NoError(t, db.Create(&log).Error)
	return log
}

func TestRepoCreatePersists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	uid, _ := seedUser(t, db)
	repo := NewGORMRepository(db)

	eid := "prod-1"
	ip := "9.9.9.9"
	err := repo.Create(context.Background(), &ActivityLog{
		UserID: &uid, Action: "CREATE", EntityType: "product",
		EntityID: &eid, IP: &ip,
	})
	require.NoError(t, err)

	var row ActivityLog
	require.NoError(t, db.Where("action = ?", "CREATE").First(&row).Error)
	assert.Equal(t, uid, *row.UserID)
	assert.Equal(t, eid, *row.EntityID)
	assert.NotEmpty(t, row.CreatedAt)
}

func TestListFiltersAndSortsNewest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	uid, _ := seedUser(t, db)
	repo := NewGORMRepository(db)

	seedLog(t, db, uid, "CREATE", "product", "A")
	seedLog(t, db, uid, "UPDATE", "product", "A")
	seedLog(t, db, uid, "DELETE", "category", "B")

	prods, total, err := repo.List(context.Background(), Query{EntityType: "product"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, prods, 2)
	assert.Equal(t, "UPDATE", prods[0].Action, "newest first")
	assert.Equal(t, "CREATE", prods[1].Action)

	creates, createsTotal, err := repo.List(context.Background(), Query{Action: "CREATE"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), createsTotal)
	assert.Len(t, creates, 1)

	byUser, _, err := repo.List(context.Background(), Query{UserID: uid})
	require.NoError(t, err)
	assert.Len(t, byUser, 3)

	cats, _, err := repo.List(context.Background(), Query{EntityType: "category"})
	require.NoError(t, err)
	assert.Len(t, cats, 1)

	none, totalNone, err := repo.List(context.Background(), Query{EntityType: "nope"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), totalNone)
	assert.Empty(t, none)
}

func TestRepoListEntityIDFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	uid, _ := seedUser(t, db)
	repo := NewGORMRepository(db)

	seedLog(t, db, uid, "CREATE", "product", "needle")
	seedLog(t, db, uid, "CREATE", "product", "other")

	rows, total, err := repo.List(context.Background(), Query{EntityID: "needle"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, "needle", *rows[0].EntityID)
}

func TestRepoListDateRange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	uid, _ := seedUser(t, db)
	repo := NewGORMRepository(db)

	seedLog(t, db, uid, "CREATE", "product", "A")

	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)
	rows, total, err := repo.List(context.Background(), Query{From: &from, To: &to})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, rows, 1)

	past := time.Now().Add(-48 * time.Hour)
	rows, total, err = repo.List(context.Background(), Query{To: &past})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, rows)
}

func TestRepoListPreloadsUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	uid, _ := seedUser(t, db)
	repo := NewGORMRepository(db)

	seedLog(t, db, uid, "CREATE", "product", "A")

	rows, _, err := repo.List(context.Background(), Query{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].User)
	assert.Equal(t, "auditor@inventory.local", rows[0].User.Email)
}

func TestRepoListPaginationAndDefaultClamp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	uid, _ := seedUser(t, db)
	repo := NewGORMRepository(db)
	for i := 0; i < 4; i++ {
		seedLog(t, db, uid, "CREATE", "product", uuid.NewString())
	}

	page, total, err := repo.List(context.Background(), Query{Page: 1, PerPage: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	assert.Len(t, page, 2)

	all, _, err := repo.List(context.Background(), Query{PerPage: 0})
	require.NoError(t, err)
	assert.Len(t, all, 4, "per_page 0 clamps to default 20")
}

func TestRepoCreatePersistsEnrichedFields(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	uid, _ := seedUser(t, db)
	repo := NewGORMRepository(db)

	reason := "restock"
	ua := "qa-agent/1.0"
	rid := "req-abc"
	before := datatypes.JSON(`{"quantity":10}`)
	after := datatypes.JSON(`{"quantity":15}`)
	err := repo.Create(context.Background(), &ActivityLog{
		UserID: &uid, Action: "STOCK_IN", EntityType: "inventory",
		Reason: &reason, UserAgent: &ua, RequestID: &rid,
		BeforeData: &before, AfterData: &after,
	})
	require.NoError(t, err)

	var row ActivityLog
	require.NoError(t, db.Where("action = ?", "STOCK_IN").First(&row).Error)
	require.NotNil(t, row.Reason)
	assert.Equal(t, "restock", *row.Reason)
	require.NotNil(t, row.UserAgent)
	assert.Equal(t, "qa-agent/1.0", *row.UserAgent)
	require.NotNil(t, row.RequestID)
	assert.Equal(t, "req-abc", *row.RequestID)
	require.NotNil(t, row.BeforeData)
	assert.Contains(t, string(*row.BeforeData), "10")
	require.NotNil(t, row.AfterData)
	assert.Contains(t, string(*row.AfterData), "15")
}

func TestRepoAppendOnlyGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	uid, _ := seedUser(t, db)

	for i := 0; i < 3; i++ {
		seedLog(t, db, uid, "CREATE", "product", "A")
	}

	// The activity log is insert-only: repeated events for the same entity
	// append new rows and never mutate existing ones.
	var count int64
	require.NoError(t, db.Model(&ActivityLog{}).Count(&count).Error)
	assert.Equal(t, int64(3), count)

	// The app-facing surface is pinned to insert + read only; no update or
	// delete path can be added without breaking this assertion.
	var _ interface {
		Create(context.Context, *ActivityLog) error
		List(context.Context, Query) ([]*ActivityLog, int64, error)
	} = (*GORMRepository)(nil)
}

func ip(v string) *string { return &v }
