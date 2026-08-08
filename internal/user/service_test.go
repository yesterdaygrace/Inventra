package user

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"inventory/internal/auth"
	sharederr "inventory/internal/shared/errors"
)

// mockRepo implements Repository for unit tests.
type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) List(ctx context.Context, q Query) ([]*User, int64, error) {
	args := m.Called(ctx, q)
	if users, ok := args.Get(0).([]*User); ok {
		return users, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	args := m.Called(ctx, id)
	if u, ok := args.Get(0).(*User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	args := m.Called(ctx, email)
	if u, ok := args.Get(0).(*User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) Update(ctx context.Context, u *User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *mockRepo) FindRoleByName(ctx context.Context, name string) (*Role, error) {
	args := m.Called(ctx, name)
	if r, ok := args.Get(0).(*Role); ok {
		return r, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) CountAdmins(ctx context.Context) (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func newSvc(repo Repository) *Service { return NewService(repo) }

func TestListPassesFilters(t *testing.T) {
	m := new(mockRepo)
	users := []*User{{Name: "Alice"}, {Name: "Bob"}}
	m.On("List", mock.Anything, mock.MatchedBy(func(q Query) bool {
		return q.Name == "ali" && q.Role == "STAFF"
	})).Return(users, int64(2), nil)

	svc := newSvc(m)
	got, total, err := svc.List(context.Background(), Query{Name: "ali", Role: "staff"})
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, int64(2), total)
	m.AssertExpectations(t)
}

func TestGet(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("FindByID", mock.Anything, id).Return(&User{ID: id, Name: "Alice"}, nil)

	got, err := newSvc(m).Get(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "Alice", got.Name)
}

func TestGetNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("FindByID", mock.Anything, id).Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).Get(context.Background(), id)
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestUpdateNameValidatesEmpty(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).UpdateName(context.Background(), uuid.New(), "   ")
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestUpdateName(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, Name: "Old"}
	m.On("FindByID", mock.Anything, id).Return(u, nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	got, err := newSvc(m).UpdateName(context.Background(), id, "  New Name  ")
	require.NoError(t, err)
	assert.Equal(t, "New Name", got.Name)
}

func TestAssignRole(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	role := &Role{Name: "ADMIN"}
	u := &User{ID: id, Name: "Alice"}
	m.On("FindRoleByName", mock.Anything, "ADMIN").Return(role, nil)
	m.On("FindByID", mock.Anything, id).Return(u, nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	got, err := newSvc(m).AssignRole(context.Background(), id, " admin ")
	require.NoError(t, err)
	assert.Equal(t, role.ID, got.RoleID)
}

func TestAssignRoleUnknownRole(t *testing.T) {
	m := new(mockRepo)
	m.On("FindRoleByName", mock.Anything, "SUPERUSER").Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).AssignRole(context.Background(), uuid.New(), "superuser")
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestActivate(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, IsActive: false}
	m.On("FindByID", mock.Anything, id).Return(u, nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	got, err := newSvc(m).Activate(context.Background(), id)
	require.NoError(t, err)
	assert.True(t, got.IsActive)
}

func TestDeactivateSelfRejected(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	_, err := newSvc(m).Deactivate(context.Background(), id, id)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestDeactivateLastAdminRejected(t *testing.T) {
	m := new(mockRepo)
	target := uuid.New()
	actor := uuid.New()
	adminRole := &Role{Name: "ADMIN"}
	u := &User{ID: target, RoleID: adminRole.ID}
	m.On("FindByID", mock.Anything, target).Return(u, nil)
	m.On("FindRoleByName", mock.Anything, "ADMIN").Return(adminRole, nil)
	m.On("CountAdmins").Return(int64(1), nil)

	_, err := newSvc(m).Deactivate(context.Background(), target, actor)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestDeactivateOK(t *testing.T) {
	m := new(mockRepo)
	target := uuid.New()
	actor := uuid.New()
	adminRole := &Role{Name: "ADMIN"}
	u := &User{ID: target, RoleID: adminRole.ID}
	m.On("FindByID", mock.Anything, target).Return(u, nil)
	m.On("FindRoleByName", mock.Anything, "ADMIN").Return(adminRole, nil)
	m.On("CountAdmins").Return(int64(2), nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	got, err := newSvc(m).Deactivate(context.Background(), target, actor)
	require.NoError(t, err)
	assert.False(t, got.IsActive)
}

func TestDeactivatePropagatesRepoError(t *testing.T) {
	m := new(mockRepo)
	target := uuid.New()
	actor := uuid.New()
	m.On("FindByID", mock.Anything, target).Return(nil, errors.New("db down"))

	_, err := newSvc(m).Deactivate(context.Background(), target, actor)
	assert.Error(t, err)
	// No repo calls beyond FindByID should happen.
	m.AssertNotCalled(t, "Update", mock.Anything)
}

func TestUpdateProfileNameOnly(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, Name: "Old", Email: "a@example.com", IsActive: true}
	m.On("FindByID", mock.Anything, id).Return(u, nil)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	got, err := newSvc(m).UpdateProfile(context.Background(), id, uuid.New(), "New", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
	assert.Equal(t, "a@example.com", got.Email)
}

func TestUpdateProfileEmailCollision(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	other := uuid.New()
	u := &User{ID: id, Email: "a@example.com"}
	m.On("FindByID", mock.Anything, id).Return(u, nil)
	m.On("FindByEmail", mock.Anything, "b@example.com").Return(&User{ID: other, Email: "b@example.com"}, nil)

	_, err := newSvc(m).UpdateProfile(context.Background(), id, uuid.New(), "", " b@example.com ", nil)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestUpdateProfileEmailOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, Email: "a@example.com"}
	m.On("FindByID", mock.Anything, id).Return(u, nil)
	m.On("FindByEmail", mock.Anything, "b@example.com").Return(nil, sharederr.ErrNotFound)
	m.On("Update", mock.Anything, mock.Anything).Return(nil)

	got, err := newSvc(m).UpdateProfile(context.Background(), id, uuid.New(), "", "b@example.com", nil)
	require.NoError(t, err)
	assert.Equal(t, "b@example.com", got.Email)
}

func TestUpdateProfileDeactivateSelf(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("FindByID", mock.Anything, id).Return(&User{ID: id}, nil)

	deactivate := false
	_, err := newSvc(m).UpdateProfile(context.Background(), id, id, "", "", &deactivate)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

// compile-time interface check
var _ Repository = (*mockRepo)(nil)
var _ = auth.User{}
