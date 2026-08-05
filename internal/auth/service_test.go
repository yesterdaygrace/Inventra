package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	sharederr "inventory/internal/shared/errors"
)

// mockRepo implements Repository for unit tests.
type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) CreateUser(u *User) error {
	args := m.Called(u)
	return args.Error(0)
}

func (m *mockRepo) FindUserByEmail(email string) (*User, error) {
	args := m.Called(email)
	if u, ok := args.Get(0).(*User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) FindUserByID(id uuid.UUID) (*User, error) {
	args := m.Called(id)
	if u, ok := args.Get(0).(*User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) UpdateUser(u *User) error {
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

func (m *mockRepo) FindRoleByID(id uuid.UUID) (*Role, error) {
	args := m.Called(id)
	if r, ok := args.Get(0).(*Role); ok {
		return r, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) CreateRefreshToken(t *RefreshToken) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *mockRepo) FindRefreshTokenByHash(hash string) (*RefreshToken, error) {
	args := m.Called(hash)
	if rt, ok := args.Get(0).(*RefreshToken); ok {
		return rt, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) UpdateRefreshToken(t *RefreshToken) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *mockRepo) CreateActivityLog(entry ActivityLogEntry) error {
	args := m.Called(entry)
	return args.Error(0)
}

var (
	staffRoleID = uuid.New()
	adminRoleID = uuid.New()
	staffRole   = &Role{ID: staffRoleID, Name: "STAFF"}
	adminRole   = &Role{ID: adminRoleID, Name: "ADMIN"}
)

func newTestService(repo Repository) *Service {
	tm := newTestManager()
	return NewService(repo, tm, bcrypt.DefaultCost)
}

func hashedPassword(pw string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	return string(h)
}

func TestRegisterCreatesStaffUser(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindUserByEmail", "ada@example.com").Return(nil, sharederr.ErrNotFound)
	repo.On("FindRoleByName", "STAFF").Return(staffRole, nil)
	repo.On("CreateUser", mock.AnythingOfType("*auth.User")).Return(nil).Run(func(args mock.Arguments) {
		u := args.Get(0).(*User)
		u.ID = uuid.New()
	})

	svc := newTestService(repo)
	user, err := svc.Register(RegisterRequest{
		Name:     "Ada",
		Email:    "Ada@Example.com",
		Password: "password123",
	})

	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", user.Email)
	assert.Equal(t, "Ada", user.Name)
	assert.Equal(t, staffRoleID, user.RoleID)
	assert.True(t, user.IsActive)
	assert.NotEqual(t, "password123", user.PasswordHash, "password must be hashed")
	repo.AssertExpectations(t)
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	existing := &User{ID: uuid.New(), Email: "ada@example.com"}
	repo := &mockRepo{}
	repo.On("FindUserByEmail", "ada@example.com").Return(existing, nil)

	svc := newTestService(repo)
	_, err := svc.Register(RegisterRequest{Name: "Ada", Email: "ada@example.com", Password: "password123"})

	assert.ErrorIs(t, err, ErrEmailTaken)
}

func TestLoginIssuesTokens(t *testing.T) {
	uid := uuid.New()
	user := &User{
		ID:           uid,
		Name:         "Ada",
		Email:        "ada@example.com",
		PasswordHash: hashedPassword("secret"),
		RoleID:       staffRoleID,
		IsActive:     true,
	}
	repo := &mockRepo{}
	repo.On("FindUserByEmail", "ada@example.com").Return(user, nil)
	repo.On("FindRoleByID", staffRoleID).Return(staffRole, nil)
	repo.On("CreateRefreshToken", mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything).Return(nil)

	svc := newTestService(repo)
	res, err := svc.Login("ada@example.com", "secret")

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NotEmpty(t, res.AccessToken)
	assert.NotEmpty(t, res.RefreshToken)
	assert.Equal(t, int64(15*60), res.ExpiresIn)
	assert.Equal(t, uid, res.User.ID)
	repo.AssertExpectations(t)
}

func TestLoginWrongPasswordUnauthorized(t *testing.T) {
	user := &User{
		ID:           uuid.New(),
		Email:        "ada@example.com",
		PasswordHash: hashedPassword("secret"),
		RoleID:       staffRoleID,
		IsActive:     true,
	}
	repo := &mockRepo{}
	repo.On("FindUserByEmail", "ada@example.com").Return(user, nil)

	svc := newTestService(repo)
	_, err := svc.Login("ada@example.com", "wrong")

	assert.ErrorIs(t, err, sharederr.ErrUnauthorized)
}

func TestLoginInactiveUserUnauthorized(t *testing.T) {
	user := &User{
		ID:           uuid.New(),
		Email:        "ada@example.com",
		PasswordHash: hashedPassword("secret"),
		RoleID:       staffRoleID,
		IsActive:     false,
	}
	repo := &mockRepo{}
	repo.On("FindUserByEmail", "ada@example.com").Return(user, nil)

	svc := newTestService(repo)
	_, err := svc.Login("ada@example.com", "secret")

	assert.ErrorIs(t, err, sharederr.ErrUnauthorized)
}

func TestRefreshRotatesToken(t *testing.T) {
	uid := uuid.New()
	user := &User{ID: uid, Email: "ada@example.com", RoleID: staffRoleID, IsActive: true}
	tm := newTestManager()
	rawToken := "some-raw-refresh-token"
	storedHash := tm.HashRefreshToken(rawToken)
	rt := &RefreshToken{
		ID:        uuid.New(),
		UserID:    uid,
		TokenHash: storedHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	repo := &mockRepo{}
	repo.On("FindRefreshTokenByHash", storedHash).Return(rt, nil)
	repo.On("FindUserByID", uid).Return(user, nil)
	repo.On("FindRoleByID", staffRoleID).Return(staffRole, nil)
	repo.On("UpdateRefreshToken", rt).Return(nil).Run(func(args mock.Arguments) {
		rt := args.Get(0).(*RefreshToken)
		require.NotNil(t, rt.RevokedAt, "old refresh token must be revoked")
	})
	repo.On("CreateRefreshToken", mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	repo.On("CreateActivityLog", mock.Anything).Return(nil)

	svc := NewService(repo, tm, bcrypt.DefaultCost)
	res, err := svc.Refresh(rawToken)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NotEmpty(t, res.AccessToken)
	assert.NotEmpty(t, res.RefreshToken)
	repo.AssertExpectations(t)
}

func TestRefreshRevokedTokenUnauthorized(t *testing.T) {
	revokedAt := time.Now()
	tm := newTestManager()
	storedHash := tm.HashRefreshToken("used")
	rt := &RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: storedHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		RevokedAt: &revokedAt,
	}
	repo := &mockRepo{}
	repo.On("FindRefreshTokenByHash", storedHash).Return(rt, nil)

	svc := newTestService(repo)
	_, err := svc.Refresh("used")

	assert.ErrorIs(t, err, sharederr.ErrUnauthorized)
}

func TestLogoutRevokesToken(t *testing.T) {
	tm := newTestManager()
	rawToken := "logout-token"
	storedHash := tm.HashRefreshToken(rawToken)
	rt := &RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: storedHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	repo := &mockRepo{}
	repo.On("FindRefreshTokenByHash", storedHash).Return(rt, nil)
	repo.On("UpdateRefreshToken", rt).Return(nil).Run(func(args mock.Arguments) {
		rt := args.Get(0).(*RefreshToken)
		require.NotNil(t, rt.RevokedAt)
	})
	repo.On("CreateActivityLog", mock.Anything).Return(nil)

	svc := newTestService(repo)
	err := svc.Logout(rawToken)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestLogoutUnknownTokenIdempotent(t *testing.T) {
	repo := &mockRepo{}
	repo.On("FindRefreshTokenByHash", mock.Anything).Return(nil, sharederr.ErrNotFound)

	svc := newTestService(repo)
	err := svc.Logout("missing")

	require.NoError(t, err)
}

func TestChangePassword(t *testing.T) {
	uid := uuid.New()
	user := &User{
		ID:           uid,
		Email:        "ada@example.com",
		PasswordHash: hashedPassword("oldpass"),
		RoleID:       staffRoleID,
	}
	repo := &mockRepo{}
	repo.On("FindUserByID", uid).Return(user, nil)
	repo.On("UpdateUser", user).Return(nil)
	repo.On("CreateActivityLog", mock.Anything).Return(nil)

	svc := newTestService(repo)
	err := svc.ChangePassword(uid, "oldpass", "newpass123")

	require.NoError(t, err)
	assert.NotEqual(t, user.PasswordHash, hashedPassword("oldpass"))
	// new hash verifies against newpass
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("newpass123")))
}

func TestChangePasswordWrongOldUnauthorized(t *testing.T) {
	uid := uuid.New()
	user := &User{
		ID:           uid,
		Email:        "ada@example.com",
		PasswordHash: hashedPassword("oldpass"),
	}
	repo := &mockRepo{}
	repo.On("FindUserByID", uid).Return(user, nil)

	svc := newTestService(repo)
	err := svc.ChangePassword(uid, "wrong", "newpass123")

	assert.ErrorIs(t, err, sharederr.ErrUnauthorized)
}

func TestUpdateProfileEmailConflict(t *testing.T) {
	uid := uuid.New()
	user := &User{ID: uid, Name: "Ada", Email: "ada@example.com", RoleID: staffRoleID}
	other := &User{ID: uuid.New(), Email: "taken@example.com"}

	repo := &mockRepo{}
	repo.On("FindUserByID", uid).Return(user, nil)
	repo.On("FindUserByEmail", "taken@example.com").Return(other, nil)

	svc := newTestService(repo)
	_, err := svc.UpdateProfile(uid, "Ada", "taken@example.com")

	assert.ErrorIs(t, err, ErrEmailTaken)
}

func TestUpdateProfileUpdatesName(t *testing.T) {
	uid := uuid.New()
	user := &User{ID: uid, Name: "Ada", Email: "ada@example.com", RoleID: staffRoleID}

	repo := &mockRepo{}
	repo.On("FindUserByID", uid).Return(user, nil)
	repo.On("UpdateUser", user).Return(nil)
	repo.On("CreateActivityLog", mock.Anything).Return(nil)

	svc := newTestService(repo)
	updated, err := svc.UpdateProfile(uid, "Ada Lovelace", "")

	require.NoError(t, err)
	assert.Equal(t, "Ada Lovelace", updated.Name)
	assert.Equal(t, "ada@example.com", updated.Email)
}

func TestRegisterEmptyFieldsValidation(t *testing.T) {
	repo := &mockRepo{}
	svc := newTestService(repo)
	_, err := svc.Register(RegisterRequest{Name: "", Email: "", Password: ""})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}
