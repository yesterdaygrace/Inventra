package category

import (
	"errors"
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

func (m *mockRepo) Create(c *Category) error {
	args := m.Called(c)
	return args.Error(0)
}

func (m *mockRepo) Get(id uuid.UUID) (*Category, error) {
	args := m.Called(id)
	if c, ok := args.Get(0).(*Category); ok {
		return c, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) Update(c *Category) error {
	args := m.Called(c)
	return args.Error(0)
}

func (m *mockRepo) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockRepo) List(q ListQuery) ([]*Category, int64, error) {
	args := m.Called(q)
	if cats, ok := args.Get(0).([]*Category); ok {
		return cats, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) CountProductsFor(id uuid.UUID) (int64, error) {
	args := m.Called(id)
	return args.Get(0).(int64), args.Error(1)
}

func newSvc(repo Repository) *Service { return NewService(repo) }

func TestCreateValidatesEmptyName(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Create("   ", nil)
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestCreateTrimsNameAndDescription(t *testing.T) {
	m := new(mockRepo)
	desc := "  electronics   "
	m.On("Create", mock.MatchedBy(func(c *Category) bool {
		return c.Name == "Electronics" && *c.Description == "electronics"
	})).Return(nil)

	got, err := newSvc(m).Create("  Electronics  ", &desc)
	require.NoError(t, err)
	assert.Equal(t, "Electronics", got.Name)
}

func TestCreateConflict(t *testing.T) {
	m := new(mockRepo)
	m.On("Create", mock.Anything).Return(sharederr.ErrConflict)
	_, err := newSvc(m).Create("Books", nil)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestGet(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", id).Return(&Category{ID: id, Name: "Books"}, nil)

	got, err := newSvc(m).Get(id)
	require.NoError(t, err)
	assert.Equal(t, "Books", got.Name)
}

func TestUpdateValidatesEmptyName(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Update(uuid.New(), " ", nil)
	assert.ErrorIs(t, err, sharederr.ErrValidation)
	m.AssertNotCalled(t, "Get", mock.Anything)
}

func TestUpdate(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	existing := &Category{ID: id, Name: "Old"}
	m.On("Get", id).Return(existing, nil)
	m.On("Update", mock.Anything).Return(nil)

	got, err := newSvc(m).Update(id, "New", nil)
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
}

func TestUpdateNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", id).Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).Update(id, "New", nil)
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestDeleteRejectsInUse(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("CountProductsFor", id).Return(int64(3), nil)

	err := newSvc(m).Delete(id)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
	m.AssertNotCalled(t, "Delete", mock.Anything)
}

func TestDeleteOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("CountProductsFor", id).Return(int64(0), nil)
	m.On("Delete", id).Return(nil)

	err := newSvc(m).Delete(id)
	assert.NoError(t, err)
}

func TestListPassesSearchAndPagination(t *testing.T) {
	m := new(mockRepo)
	cats := []*Category{{Name: "Books"}}
	m.On("List", mock.MatchedBy(func(q ListQuery) bool {
		return q.Search == "boo" && q.Page == 2
	})).Return(cats, int64(1), nil)

	got, total, err := newSvc(m).List(ListQuery{Search: "boo", Page: 2})
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, int64(1), total)
}

func TestListPropagatesError(t *testing.T) {
	m := new(mockRepo)
	m.On("List", mock.Anything).Return(nil, int64(0), errors.New("db down"))

	_, _, err := newSvc(m).List(ListQuery{})
	assert.Error(t, err)
}

var _ Repository = (*mockRepo)(nil)
