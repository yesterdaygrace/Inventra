package product

import (
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

func (m *mockRepo) Create(p *Product) error {
	args := m.Called(p)
	return args.Error(0)
}

func (m *mockRepo) Get(id uuid.UUID) (*Product, error) {
	args := m.Called(id)
	if p, ok := args.Get(0).(*Product); ok {
		return p, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) Update(p *Product) error {
	args := m.Called(p)
	return args.Error(0)
}

func (m *mockRepo) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockRepo) List(q ListQuery) ([]*Product, int64, error) {
	args := m.Called(q)
	if ps, ok := args.Get(0).([]*Product); ok {
		return ps, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) SKUExists(sku string, excludeID uuid.UUID) (bool, error) {
	args := m.Called(sku, excludeID)
	return args.Bool(0), args.Error(1)
}

func newSvc(repo Repository) *Service { return NewService(repo) }

func TestCreateValidatesEmptyFields(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Create("  ", "SKU-1", nil, 10, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestCreateRejectsNegativePrice(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Create("Widget", "SKU-1", nil, -1, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestCreateDuplicateSKUConflict(t *testing.T) {
	m := new(mockRepo)
	m.On("SKUExists", "DUPE", uuid.Nil).Return(true, nil)

	_, err := newSvc(m).Create("Widget", "DUPE", nil, 10, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestCreateOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("SKUExists", "WID-1", uuid.Nil).Return(false, nil)
	m.On("Create", mock.MatchedBy(func(p *Product) bool {
		return p.Name == "Widget" && p.SKU == "WID-1" && p.Price == 12.5
	})).Return(nil).Run(func(args mock.Arguments) {
		args.Get(0).(*Product).ID = id
	})

	p, err := newSvc(m).Create("Widget", "WID-1", nil, 12.5, uuid.New(), 5, false)
	require.NoError(t, err)
	assert.Equal(t, id, p.ID)
}

func TestUpdateDuplicateSKUExcludingSelf(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("SKUExists", "KEEP", id).Return(false, nil)
	m.On("Get", id).Return(&Product{ID: id, SKU: "OLD", Name: "Old"}, nil)
	m.On("Update", mock.Anything).Return(nil)

	p, err := newSvc(m).Update(id, "New", "KEEP", nil, 10, uuid.New(), 5, false)
	require.NoError(t, err)
	assert.Equal(t, "KEEP", p.SKU)
}

func TestUpdateConflictWhenTakenByOther(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("SKUExists", "TAKEN", id).Return(true, nil)

	_, err := newSvc(m).Update(id, "New", "TAKEN", nil, 10, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestUpdateNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("SKUExists", "NEW", id).Return(false, nil)
	m.On("Get", id).Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).Update(id, "New", "NEW", nil, 10, uuid.New(), 5, false)
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestDelete(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Delete", id).Return(nil)

	err := newSvc(m).Delete(id)
	assert.NoError(t, err)
}

func TestListRejectsInjectionSort(t *testing.T) {
	m := new(mockRepo)
	_, _, err := newSvc(m).List(ListQuery{Sort: "name; DROP TABLE products"})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestListRejectsUnknownSortColumn(t *testing.T) {
	m := new(mockRepo)
	_, _, err := newSvc(m).List(ListQuery{Sort: "unknown_col"})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestListPassesWhitelistedSorts(t *testing.T) {
	for _, sort := range []string{"name", "-name", "price", "-price", "created_at", "sku", "-sku", ""} {
		m := new(mockRepo)
		m.On("List", mock.MatchedBy(func(q ListQuery) bool { return q.Sort == sort })).
			Return([]*Product{}, int64(0), nil)

		_, _, err := newSvc(m).List(ListQuery{Sort: sort})
		assert.NoError(t, err, "sort %q should be accepted", sort)
	}
}

func TestListTrimsSearch(t *testing.T) {
	m := new(mockRepo)
	m.On("List", mock.MatchedBy(func(q ListQuery) bool { return q.Q == "widget" })).
		Return([]*Product{}, int64(0), nil)

	_, _, err := newSvc(m).List(ListQuery{Q: "  widget  "})
	assert.NoError(t, err)
}

func TestGetReturnsProduct(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", id).Return(&Product{ID: id, Name: "Widget"}, nil)

	p, err := newSvc(m).Get(id)
	require.NoError(t, err)
	assert.Equal(t, "Widget", p.Name)
}

func TestGetNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", id).Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).Get(id)
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
