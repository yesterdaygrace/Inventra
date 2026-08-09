package category

import (
	"context"
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

func (m *mockRepo) Create(ctx context.Context, c *Category) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *mockRepo) Get(ctx context.Context, id uuid.UUID) (*Category, error) {
	args := m.Called(ctx, id)
	if c, ok := args.Get(0).(*Category); ok {
		return c, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) Update(ctx context.Context, c *Category) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepo) List(ctx context.Context, q ListQuery) ([]*Category, int64, error) {
	args := m.Called(ctx, q)
	if cats, ok := args.Get(0).([]*Category); ok {
		return cats, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) CountProductsFor(ctx context.Context, id uuid.UUID) (int64, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(int64), args.Error(1)
}

func newSvc(repo Repository) *Service { return NewService(repo) }

func strPtr(s string) *string { return &s }
func bPtr(b bool) *bool       { return &b }

func TestCreateValidatesEmptyName(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Create(context.Background(), "   ", nil)
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestCreateTrimsNameAndDescription(t *testing.T) {
	m := new(mockRepo)
	desc := "  electronics   "
	m.On("Create", mock.Anything, mock.MatchedBy(func(c *Category) bool {
		return c.Name == "Electronics" && *c.Description == "electronics"
	})).Return(nil)

	got, err := newSvc(m).Create(context.Background(), "  Electronics  ", &desc)
	require.NoError(t, err)
	assert.Equal(t, "Electronics", got.Name)
}

func TestCreateConflict(t *testing.T) {
	m := new(mockRepo)
	m.On("Create", mock.Anything, mock.Anything).Return(sharederr.ErrConflict)
	_, err := newSvc(m).Create(context.Background(), "Books", nil)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestGet(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(&Category{ID: id, Name: "Books"}, nil)

	got, err := newSvc(m).Get(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "Books", got.Name)
}

func TestUpdateValidatesEmptyName(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Update(context.Background(), uuid.New(), UpdateParams{Name: strPtr(" ")})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
	m.AssertNotCalled(t, "Get", mock.Anything)
}

func TestUpdate(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	existing := &Category{ID: id, Name: "Old"}
	m.On("Get", mock.Anything, id).Return(existing, nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	got, err := newSvc(m).Update(context.Background(), id, UpdateParams{Name: strPtr("New")})
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
}

func TestUpdateNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).Update(context.Background(), id, UpdateParams{Name: strPtr("New")})
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestUpdateDescriptionOnlyKeepsName(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	existing := &Category{ID: id, Name: "Books", Description: strPtr("old desc")}
	m.On("Get", mock.Anything, id).Return(existing, nil)
	m.On("Update", mock.Anything, mock.MatchedBy(func(c *Category) bool {
		return c.Name == "Books" && c.Description != nil && *c.Description == "new desc"
	})).Return(nil)

	got, err := newSvc(m).Update(context.Background(), id, UpdateParams{Description: strPtr("new desc")})
	require.NoError(t, err)
	assert.Equal(t, "Books", got.Name)
	assert.Equal(t, "new desc", *got.Description)
}

func TestUpdateActiveOnlyKeepsName(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	existing := &Category{ID: id, Name: "Books", IsActive: true}
	m.On("Get", mock.Anything, id).Return(existing, nil)
	m.On("Update", mock.Anything, mock.MatchedBy(func(c *Category) bool {
		return !c.IsActive && c.Name == "Books"
	})).Return(nil)

	got, err := newSvc(m).Update(context.Background(), id, UpdateParams{IsActive: bPtr(false)})
	require.NoError(t, err)
	assert.False(t, got.IsActive)
	assert.Equal(t, "Books", got.Name)
}

func TestUpdateClearsDescriptionToNil(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	existing := &Category{ID: id, Name: "C", Description: strPtr("keep")}
	m.On("Get", mock.Anything, id).Return(existing, nil)
	m.On("Update", mock.Anything, mock.MatchedBy(func(c *Category) bool {
		return c.Description == nil
	})).Return(nil)

	got, err := newSvc(m).Update(context.Background(), id, UpdateParams{Description: strPtr("")})
	require.NoError(t, err)
	assert.Nil(t, got.Description)
}

func TestDeleteRejectsInUse(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("CountProductsFor", mock.Anything, id).Return(int64(3), nil)

	err := newSvc(m).Delete(context.Background(), id)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
	m.AssertNotCalled(t, "Delete", mock.Anything)
}

func TestDeleteOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("CountProductsFor", mock.Anything, id).Return(int64(0), nil)
	m.On("Delete", mock.Anything, id).Return(nil)

	err := newSvc(m).Delete(context.Background(), id)
	assert.NoError(t, err)
}

func TestListPassesSearchAndPagination(t *testing.T) {
	m := new(mockRepo)
	cats := []*Category{{Name: "Books"}}
	m.On("List", mock.Anything, mock.MatchedBy(func(q ListQuery) bool {
		return q.Search == "boo" && q.Page == 2
	})).Return(cats, int64(1), nil)

	got, total, err := newSvc(m).List(context.Background(), ListQuery{Search: "boo", Page: 2})
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, int64(1), total)
}

func TestListPropagatesError(t *testing.T) {
	m := new(mockRepo)
	m.On("List", mock.Anything, mock.Anything).Return(nil, int64(0), errors.New("db down"))

	_, _, err := newSvc(m).List(context.Background(), ListQuery{})
	assert.Error(t, err)
}

var _ Repository = (*mockRepo)(nil)
