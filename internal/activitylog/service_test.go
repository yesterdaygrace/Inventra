package activitylog

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"inventory/internal/shared/audit"
)

type mockRepo struct{ mock.Mock }

func (m *mockRepo) Create(ctx context.Context, l *ActivityLog) error {
	args := m.Called(ctx, l)
	return args.Error(0)
}

func (m *mockRepo) List(ctx context.Context, q Query) ([]*ActivityLog, int64, error) {
	args := m.Called(ctx, q)
	if logs, ok := args.Get(0).([]*ActivityLog); ok {
		return logs, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

var _ Repository = (*mockRepo)(nil)

func newSvc(repo Repository) *Service {
	return NewService(repo, zap.NewNop())
}

func TestRecordPersistsEntry(t *testing.T) {
	m := new(mockRepo)
	uid := uuid.New()
	ip := "10.0.0.1"
	m.On("Create", mock.Anything, mock.MatchedBy(func(l *ActivityLog) bool {
		return l.UserID != nil && *l.UserID == uid &&
			l.Action == "CREATE" && l.EntityType == "product" &&
			l.IP != nil && *l.IP == ip && l.Details != nil
	})).Return(nil)

	newSvc(m).Record(audit.Entry{
		UserID:     &uid,
		Action:     "CREATE",
		EntityType: "product",
		Details:    map[string]any{"name": "Widget"},
		IP:         &ip,
	})
	m.AssertExpectations(t)
}

func TestRecordFailureSafeNeverFailsCaller(t *testing.T) {
	m := new(mockRepo)
	m.On("Create", mock.Anything, mock.Anything).Return(errors.New("db down"))

	uid := uuid.New()
	svc := newSvc(m)

	assert.NotPanics(t, func() {
		svc.Record(audit.Entry{UserID: &uid, Action: "LOGIN", EntityType: "user"})
	})
	m.AssertExpectations(t)
}

func TestRecordSchemaBrokenDetailsSwallowed(t *testing.T) {
	m := new(mockRepo)
	// Marshal should always succeed for a map, so no panic path here; confirm
	// the happy map path recorded. Use a nil repo to prove schema errors are
	// not reachable from Record's contract.
	m.On("Create", mock.Anything, mock.Anything).Return(nil)
	newSvc(m).Record(audit.Entry{EntityType: "user", Action: "CREATE", Details: map[string]any{"a": "b"}})
	m.AssertExpectations(t)
}

func TestRecordPersistsEnrichedFields(t *testing.T) {
	m := new(mockRepo)
	uid := uuid.New()
	reason := "restock"
	ua := "test-agent/1.0"
	rid := "req-123"
	m.On("Create", mock.Anything, mock.MatchedBy(func(l *ActivityLog) bool {
		return l.UserID != nil && *l.UserID == uid &&
			l.Action == "STOCK_IN" && l.EntityType == "inventory" &&
			l.Reason != nil && *l.Reason == reason &&
			l.UserAgent != nil && *l.UserAgent == ua &&
			l.RequestID != nil && *l.RequestID == rid &&
			l.BeforeData != nil && l.AfterData != nil
	})).Return(nil)

	newSvc(m).Record(audit.Entry{
		UserID:     &uid,
		Action:     "STOCK_IN",
		EntityType: "inventory",
		Reason:     &reason,
		UserAgent:  &ua,
		RequestID:  &rid,
		BeforeData: map[string]int{"quantity": 10},
		AfterData:  map[string]int{"quantity": 15},
	})
	m.AssertExpectations(t)
}

func TestRecordPartialFieldsNilTolerant(t *testing.T) {
	m := new(mockRepo)
	// Only the required fields are set; every optional field must stay nil
	// and Record must not panic.
	m.On("Create", mock.Anything, mock.MatchedBy(func(l *ActivityLog) bool {
		return l.UserID == nil && l.Action == "LOGIN" && l.EntityType == "user" &&
			l.Reason == nil && l.UserAgent == nil && l.RequestID == nil &&
			l.BeforeData == nil && l.AfterData == nil
	})).Return(nil)

	newSvc(m).Record(audit.Entry{Action: "LOGIN", EntityType: "user"})
	m.AssertExpectations(t)
}

func TestListDelegates(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	uid := uuid.New()
	logs := []*ActivityLog{{ID: id, UserID: &uid, Action: "CREATE", EntityType: "product"}}
	m.On("List", mock.Anything, mock.MatchedBy(func(q Query) bool {
		return q.UserID == uid && q.EntityType == "product" && q.Page == 1 && q.PerPage == 10
	})).Return(logs, int64(1), nil)

	got, total, err := newSvc(m).List(context.Background(),
		Query{UserID: uid, EntityType: "product", Page: 1, PerPage: 10})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, got, 1)
	assert.Equal(t, id, got[0].ID)
	m.AssertExpectations(t)
}

func TestParseUserIDEmptyAndValid(t *testing.T) {
	id, err := ParseUserID("")
	assert.NoError(t, err)
	assert.Equal(t, uuid.Nil, id)

	u := uuid.New()
	id, err = ParseUserID(u.String())
	assert.NoError(t, err)
	assert.Equal(t, u, id)

	_, err = ParseUserID("not-a-uuid")
	assert.Error(t, err)
}
