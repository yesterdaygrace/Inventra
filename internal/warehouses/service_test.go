package warehouses

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	sharederr "inventory/internal/shared/errors"
)

// mockRepo implements Repository for handler tests without a real database.
type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) Create(ctx context.Context, w *Warehouse) error {
	args := m.Called(ctx, w)
	return args.Error(0)
}

func (m *mockRepo) Get(ctx context.Context, id uuid.UUID) (*Warehouse, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*Warehouse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) GetByCode(ctx context.Context, code string) (*Warehouse, error) {
	args := m.Called(ctx, code)
	if v := args.Get(0); v != nil {
		return v.(*Warehouse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) Update(ctx context.Context, w *Warehouse) error {
	args := m.Called(ctx, w)
	return args.Error(0)
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepo) List(ctx context.Context, q ListQuery) ([]*Warehouse, int64, error) {
	args := m.Called(ctx, q)
	return args.Get(0).([]*Warehouse), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) CountInventoryFor(ctx context.Context, id uuid.UUID) (int64, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockRepo) ListWithInventoryCount(ctx context.Context, q ListQuery) ([]*Warehouse, int64, error) {
	args := m.Called(ctx, q)
	return args.Get(0).([]*Warehouse), args.Get(1).(int64), args.Error(2)
}

func TestService_ListPassesThrough(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	repo := NewGORMRepository(db)
	svc := NewService(repo)

	desc := "hub"
	_, err := svc.Create(context.Background(), "HUB", "Hub", &desc)
	require.NoError(t, err)
	whs, total, err := svc.List(context.Background(), ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, whs, 1)
	assert.Equal(t, "HUB", whs[0].Code)
}

func TestService_CreateValidation(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	svc := NewService(NewGORMRepository(db))

	_, err := svc.Create(context.Background(), "  ", "Name", nil)
	assert.ErrorIs(t, err, sharederr.ErrValidation)

	_, err = svc.Create(context.Background(), "OK", "  ", nil)
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestService_UpdateValidationAndPartialUpdate(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	svc := NewService(NewGORMRepository(db))

	w, err := svc.Create(context.Background(), "OLD", "Old Name", nil)
	require.NoError(t, err)

	// empty name rejected
	_, err = svc.Update(context.Background(), w.ID, UpdateParams{Name: strPtr("  ")})
	assert.ErrorIs(t, err, sharederr.ErrValidation)

	// partial update only touches provided fields
	newName := "New Name"
	got, err := svc.Update(context.Background(), w.ID, UpdateParams{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, "New Name", got.Name)
	assert.Equal(t, "OLD", got.Code)

	// unknown id
	_, err = svc.Update(context.Background(), uuid.New(), UpdateParams{Name: &newName})
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestService_DeleteConflictWhenInventoryReferenced(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(append(testModels, &testInventoryRow{})...))
	svc := NewService(NewGORMRepository(db))

	w, err := svc.Create(context.Background(), "BUSY", "Busy", nil)
	require.NoError(t, err)

	// no rows -> delete (soft-deactivate) succeeds
	require.NoError(t, svc.Delete(context.Background(), w.ID))

	// reference rows -> conflict
	w2, err := svc.Create(context.Background(), "BUSY2", "Busy2", nil)
	require.NoError(t, err)
	require.NoError(t, db.Create(&testInventoryRow{WarehouseID: w2.ID}).Error)
	err = svc.Delete(context.Background(), w2.ID)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestService_GetByCodeOrNotFound(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(testModels...))
	svc := NewService(NewGORMRepository(db))

	_, err := svc.Create(context.Background(), "DEFAULT", "Default", nil)
	require.NoError(t, err)

	w, err := svc.GetByCode(context.Background(), "DEFAULT")
	require.NoError(t, err)
	assert.Equal(t, "Default", w.Name)

	_, err = svc.GetByCode(context.Background(), "MISSING")
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func strPtr(s string) *string { return &s }
