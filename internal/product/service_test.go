package product

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	sharederr "inventory/internal/shared/errors"
)

// mockRepo implements Repository for unit tests.
type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) Create(ctx context.Context, p *Product) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockRepo) Get(ctx context.Context, id uuid.UUID) (*Product, error) {
	args := m.Called(ctx, id)
	if p, ok := args.Get(0).(*Product); ok {
		return p, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) Update(ctx context.Context, p *Product) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepo) List(ctx context.Context, q ListQuery) ([]*Product, int64, error) {
	args := m.Called(ctx, q)
	if ps, ok := args.Get(0).([]*Product); ok {
		return ps, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) SKUExists(ctx context.Context, sku string, excludeID uuid.UUID) (bool, error) {
	args := m.Called(ctx, sku, excludeID)
	return args.Bool(0), args.Error(1)
}

func newSvc(repo Repository) *Service { return NewService(repo) }

func TestCreateValidatesEmptyFields(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Create(context.Background(), "  ", "SKU-1", nil, 10, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestCreateRejectsNegativePrice(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Create(context.Background(), "Widget", "SKU-1", nil, -1, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestCreateDuplicateSKUConflict(t *testing.T) {
	m := new(mockRepo)
	m.On("SKUExists", mock.Anything, "DUPE", uuid.Nil).Return(true, nil)

	_, err := newSvc(m).Create(context.Background(), "Widget", "DUPE", nil, 10, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestCreateOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("SKUExists", mock.Anything, "WID-1", uuid.Nil).Return(false, nil)
	m.On("Create", mock.Anything, mock.MatchedBy(func(p *Product) bool {
		return p.Name == "Widget" && p.SKU == "WID-1" && p.Price == 12.5
	})).Return(nil).Run(func(args mock.Arguments) {
		args.Get(1).(*Product).ID = id
	})

	p, err := newSvc(m).Create(context.Background(), "Widget", "WID-1", nil, 12.5, uuid.New(), 5, false)
	require.NoError(t, err)
	assert.Equal(t, id, p.ID)
}

func TestUpdateDuplicateSKUExcludingSelf(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("SKUExists", mock.Anything, "KEEP", id).Return(false, nil)
	m.On("Get", mock.Anything, id).Return(&Product{ID: id, SKU: "OLD", Name: "Old"}, nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	p, err := newSvc(m).Update(context.Background(), id, "New", "KEEP", nil, 10, uuid.New(), 5, false)
	require.NoError(t, err)
	assert.Equal(t, "KEEP", p.SKU)
}

func TestUpdateConflictWhenTakenByOther(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("SKUExists", mock.Anything, "TAKEN", id).Return(true, nil)

	_, err := newSvc(m).Update(context.Background(), id, "New", "TAKEN", nil, 10, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestUpdateNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("SKUExists", mock.Anything, "NEW", id).Return(false, nil)
	m.On("Get", mock.Anything, id).Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).Update(context.Background(), id, "New", "NEW", nil, 10, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestDelete(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Delete", mock.Anything, id).Return(nil)

	err := newSvc(m).Delete(context.Background(), id)
	assert.NoError(t, err)
}

func TestListRejectsInjectionSort(t *testing.T) {
	m := new(mockRepo)
	_, _, err := newSvc(m).List(context.Background(), ListQuery{Sort: "name; DROP TABLE products"})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestListRejectsUnknownSortColumn(t *testing.T) {
	m := new(mockRepo)
	_, _, err := newSvc(m).List(context.Background(), ListQuery{Sort: "unknown_col"})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestListPassesWhitelistedSorts(t *testing.T) {
	for _, sort := range []string{"name", "-name", "price", "-price", "created_at", "sku", "-sku", ""} {
		m := new(mockRepo)
		m.On("List", mock.Anything, mock.MatchedBy(func(q ListQuery) bool { return q.Sort == sort })).
			Return([]*Product{}, int64(0), nil)

		_, _, err := newSvc(m).List(context.Background(), ListQuery{Sort: sort})
		assert.NoError(t, err, "sort %q should be accepted", sort)
	}
}

func TestListTrimsSearch(t *testing.T) {
	m := new(mockRepo)
	m.On("List", mock.Anything, mock.MatchedBy(func(q ListQuery) bool { return q.Q == "widget" })).
		Return([]*Product{}, int64(0), nil)

	_, _, err := newSvc(m).List(context.Background(), ListQuery{Q: "  widget  "})
	assert.NoError(t, err)
}

func TestGetReturnsProduct(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(&Product{ID: id, Name: "Widget"}, nil)

	p, err := newSvc(m).Get(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "Widget", p.Name)
}

func TestGetNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).Get(context.Background(), id)
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestTrimDescription(t *testing.T) {
	empty := "   "
	assert.Nil(t, trimDescription(&empty))

	spaced := "  useful  "
	got := trimDescription(&spaced)
	require.NotNil(t, got)
	assert.Equal(t, "useful", *got)
}

var _ Repository = (*mockRepo)(nil)
