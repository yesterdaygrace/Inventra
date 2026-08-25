package inventory

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

func (m *mockRepo) Receive(ctx context.Context, mv Movement) (*Inventory, error) {
	args := m.Called(ctx, mv)
	if inv, ok := args.Get(0).(*Inventory); ok {
		return inv, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) Issue(ctx context.Context, mv Movement) (*Inventory, error) {
	args := m.Called(ctx, mv)
	if inv, ok := args.Get(0).(*Inventory); ok {
		return inv, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) Transfer(ctx context.Context, t Transfer) (*Inventory, error) {
	args := m.Called(ctx, t)
	if inv, ok := args.Get(0).(*Inventory); ok {
		return inv, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) DefaultWarehouse(ctx context.Context) (uuid.UUID, error) {
	args := m.Called(ctx)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *mockRepo) List(ctx context.Context, q ListQuery) ([]*InventoryView, int64, error) {
	args := m.Called(ctx, q)
	if views, ok := args.Get(0).([]*InventoryView); ok {
		return views, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) ApplyCorrection(ctx context.Context, productID, warehouseID uuid.UUID, targetQuantity int, referenceType, referenceID, reason string, userID *uuid.UUID) (*Inventory, error) {
	args := m.Called(ctx, productID, warehouseID, targetQuantity)
	if inv, ok := args.Get(0).(*Inventory); ok {
		return inv, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) CreateReservation(ctx context.Context, rsv Reservation) (*Reservation, error) {
	args := m.Called(ctx, rsv)
	if r, ok := args.Get(0).(*Reservation); ok {
		return r, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) ReleaseReservation(ctx context.Context, id uuid.UUID) (*Reservation, error) {
	args := m.Called(ctx, id)
	if r, ok := args.Get(0).(*Reservation); ok {
		return r, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) ConsumeReservation(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*Reservation, *Inventory, error) {
	args := m.Called(ctx, id, userID)
	if r, ok := args.Get(0).(*Reservation); ok {
		var inv *Inventory
		if i, ok2 := args.Get(1).(*Inventory); ok2 {
			inv = i
		}
		return r, inv, args.Error(2)
	}
	return nil, nil, args.Error(2)
}

func (m *mockRepo) Reservations(ctx context.Context, q ReservationQuery) ([]*ReservationView, int64, error) {
	args := m.Called(ctx, q)
	if views, ok := args.Get(0).([]*ReservationView); ok {
		return views, args.Get(1).(int64), args.Error(2)
	}
	return nil, 0, args.Error(2)
}

func (m *mockRepo) Ledger(ctx context.Context, q LedgerQuery) ([]*LedgerView, int64, error) {
	args := m.Called(ctx, q)
	if views, ok := args.Get(0).([]*LedgerView); ok {
		return views, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

func newSvc(repo Repository) *Service { return NewService(repo) }

func TestStockInValidatesProduct(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Receive(context.Background(), Movement{ProductID: uuid.Nil, Type: LedgerReceive, Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestStockInValidatesType(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Receive(context.Background(), Movement{ProductID: uuid.New(), Type: "SIDE", Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestStockInValidatesQuantity(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).Receive(context.Background(), Movement{ProductID: uuid.New(), Type: LedgerReceive, Quantity: 0})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestStockInDelegates(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("Receive", mock.Anything, mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == LedgerReceive && mv.Quantity == 10
	})).Return(&Inventory{ProductID: pid, Quantity: 10}, nil)

	inv, err := newSvc(m).Receive(context.Background(), Movement{ProductID: pid, Type: LedgerReceive, Quantity: 10})
	require.NoError(t, err)
	assert.Equal(t, 10, inv.Quantity)
}

func TestStockOutOverdrawConflict(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("Issue", mock.Anything, mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == LedgerIssue && mv.Quantity == 50
	})).Return(nil, sharederr.ErrConflict)

	_, err := newSvc(m).Issue(context.Background(), Movement{ProductID: pid, Type: LedgerIssue, Quantity: 50})
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestStockOutDelegates(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("Issue", mock.Anything, mock.Anything).Return(&Inventory{ProductID: pid, Quantity: 4}, nil)

	inv, err := newSvc(m).Issue(context.Background(), Movement{ProductID: pid, Type: LedgerIssue, Quantity: 6})
	require.NoError(t, err)
	assert.Equal(t, 4, inv.Quantity)
}

func TestTransferValidatesFields(t *testing.T) {
	m := new(mockRepo)
	svc := newSvc(m)
	pid := uuid.New()
	w1 := uuid.New()
	w2 := uuid.New()

	_, err := svc.Transfer(context.Background(), Transfer{ProductID: uuid.Nil, FromWarehouseID: w1, ToWarehouseID: w2, Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)

	_, err = svc.Transfer(context.Background(), Transfer{ProductID: pid, FromWarehouseID: uuid.Nil, ToWarehouseID: w2, Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)

	_, err = svc.Transfer(context.Background(), Transfer{ProductID: pid, FromWarehouseID: w1, ToWarehouseID: uuid.Nil, Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)

	_, err = svc.Transfer(context.Background(), Transfer{ProductID: pid, FromWarehouseID: w1, ToWarehouseID: w2, Quantity: 0})
	assert.ErrorIs(t, err, sharederr.ErrValidation)

	_, err = svc.Transfer(context.Background(), Transfer{ProductID: pid, FromWarehouseID: w1, ToWarehouseID: w1, Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestTransferDelegates(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	w1 := uuid.New()
	w2 := uuid.New()
	m.On("Transfer", mock.Anything, mock.MatchedBy(func(t Transfer) bool {
		return t.ProductID == pid && t.FromWarehouseID == w1 && t.ToWarehouseID == w2 && t.Quantity == 3
	})).Return(&Inventory{ProductID: pid, Quantity: 3}, nil)

	inv, err := newSvc(m).Transfer(context.Background(), Transfer{
		ProductID:       pid,
		FromWarehouseID: w1,
		ToWarehouseID:   w2,
		Quantity:        3,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, inv.Quantity)
}

var _ Repository = (*mockRepo)(nil)
