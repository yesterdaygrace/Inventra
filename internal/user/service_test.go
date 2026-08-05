package user

import (
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

func (m *mockRepo) List(q Query) ([]*User, int64, error) {
	args := m.Called(q)
	if users, ok := args.Get(0).([]*User); ok {
		return users, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) FindByID(id uuid.UUID) (*User, error) {
	args := m.Called(id)
	if u, ok := args.Get(0).(*User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) FindByEmail(email string) (*User, error) {
	args := m.Called(email)
	if u, ok := args.Get(0).(*User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) Update(u *User) error {
	args := m.Called(u)
	return args.Error(0)
}

func (m *mockRepo) FindRoleByName(name string) (*Role, error) {
	args := m.Called(name)
	if r, ok := args.Get(0).(*Role); ok {
		return r, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) CountAdmins() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func newSvc(repo Repository) *Service { return NewService(repo) }

func TestListPassesFilters(t *testing.T) {
	m := new(mockRepo)
	users := []*User{{Name: "Alice"}, {Name: "Bob"}}
	m.On("List", mock.MatchedBy(func(q Query) bool {
		return q.Name == "ali" && q.Role == "STAFF"
	})).Return(users, int64(2), nil)

	svc := newSvc(m)
	got, total, err := svc.List(Query{Name: "ali", Role: "staff"})
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, int64(2), total)
	m.AssertExpectations(t)
}

func TestGet(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("FindByID", id).Return(&User{ID: id, Name: "Alice"}, nil)

	got, err := newSvc(m).Get(id)
	require.NoError(t, err)
	assert.Equal(t, "Alice", got.Name)
}

func TestGetNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("FindByID", id).Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).Get(id)
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestUpdateNameValidatesEmpty(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).UpdateName(uuid.New(), "   ")
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestUpdateName(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, Name: "Old"}
	m.On("FindByID", id).Return(u, nil)
	m.On("Update", mock.Anything).Return(nil)

	got, err := newSvc(m).UpdateName(id, "  New Name  ")
	require.NoError(t, err)
	assert.Equal(t, "New Name", got.Name)
}

func TestAssignRole(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	role := &Role{Name: "ADMIN"}
	u := &User{ID: id, Name: "Alice"}
	m.On("FindRoleByName", "ADMIN").Return(role, nil)
	m.On("FindByID", id).Return(u, nil)
	m.On("Update", mock.Anything).Return(nil)

	got, err := newSvc(m).AssignRole(id, " admin ")
	require.NoError(t, err)
	assert.Equal(t, role.ID, got.RoleID)
}

func TestAssignRoleUnknownRole(t *testing.T) {
	m := new(mockRepo)
	m.On("FindRoleByName", "SUPERUSER").Return(nil, sharederr.ErrNotFound)

	_, err := newSvc(m).AssignRole(uuid.New(), "superuser")
	assert.ErrorIs(t, err, sharederr.ErrNotFound)
}

func TestActivate(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, IsActive: false}
	m.On("FindByID", id).Return(u, nil)
	m.On("Update", mock.Anything).Return(nil)

	got, err := newSvc(m).Activate(id)
	require.NoError(t, err)
	assert.True(t, got.IsActive)
}

func TestDeactivateSelfRejected(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	_, err := newSvc(m).Deactivate(id, id)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestDeactivateLastAdminRejected(t *testing.T) {
	m := new(mockRepo)
	target := uuid.New()
	actor := uuid.New()
	adminRole := &Role{Name: "ADMIN"}
	u := &User{ID: target, RoleID: adminRole.ID}
	m.On("FindByID", target).Return(u, nil)
	m.On("FindRoleByName", "ADMIN").Return(adminRole, nil)
	m.On("CountAdmins").Return(int64(1), nil)

	_, err := newSvc(m).Deactivate(target, actor)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestDeactivateOK(t *testing.T) {
	m := new(mockRepo)
	target := uuid.New()
	actor := uuid.New()
	adminRole := &Role{Name: "ADMIN"}
	u := &User{ID: target, RoleID: adminRole.ID}
	m.On("FindByID", target).Return(u, nil)
	m.On("FindRoleByName", "ADMIN").Return(adminRole, nil)
	m.On("CountAdmins").Return(int64(2), nil)
	m.On("Update", mock.Anything).Return(nil)

	got, err := newSvc(m).Deactivate(target, actor)
	require.NoError(t, err)
	assert.False(t, got.IsActive)
}

func TestDeactivatePropagatesRepoError(t *testing.T) {
	m := new(mockRepo)
	target := uuid.New()
	actor := uuid.New()
	m.On("FindByID", target).Return(nil, errors.New("db down"))

	_, err := newSvc(m).Deactivate(target, actor)
	assert.Error(t, err)
	// No repo calls beyond FindByID should happen.
	m.AssertNotCalled(t, "Update", mock.Anything)
}

func TestUpdateProfileNameOnly(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, Name: "Old", Email: "a@example.com", IsActive: true}
	m.On("FindByID", id).Return(u, nil)
	m.On("Update", mock.Anything).Return(nil)

	got, err := newSvc(m).UpdateProfile(id, uuid.New(), "New", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
	assert.Equal(t, "a@example.com", got.Email)
}

func TestUpdateProfileEmailCollision(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	other := uuid.New()
	u := &User{ID: id, Email: "a@example.com"}
	m.On("FindByID", id).Return(u, nil)
	m.On("FindByEmail", "b@example.com").Return(&User{ID: other, Email: "b@example.com"}, nil)

	_, err := newSvc(m).UpdateProfile(id, uuid.New(), "", " b@example.com ", nil)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestUpdateProfileEmailOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	u := &User{ID: id, Email: "a@example.com"}
	m.On("FindByID", id).Return(u, nil)
	m.On("FindByEmail", "b@example.com").Return(nil, sharederr.ErrNotFound)
	m.On("Update", mock.Anything).Return(nil)

	got, err := newSvc(m).UpdateProfile(id, uuid.New(), "", "b@example.com", nil)
	require.NoError(t, err)
	assert.Equal(t, "b@example.com", got.Email)
}

func TestUpdateProfileDeactivateSelf(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("FindByID", id).Return(&User{ID: id}, nil)

	deactivate := false
	_, err := newSvc(m).UpdateProfile(id, id, "", "", &deactivate)
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

// compile-time interface check
var _ Repository = (*mockRepo)(nil)
var _ = auth.User{}
