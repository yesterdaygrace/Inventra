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

func (m *mockRepo) StockIn(ctx context.Context, mv Movement) (*Inventory, error) {
	args := m.Called(ctx, mv)
	if inv, ok := args.Get(0).(*Inventory); ok {
		return inv, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) StockOut(ctx context.Context, mv Movement) (*Inventory, error) {
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

func (m *mockRepo) Transactions(ctx context.Context, q TransactionQuery) ([]*TransactionView, int64, error) {
	args := m.Called(ctx, q)
	if views, ok := args.Get(0).([]*TransactionView); ok {
		return views, args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}

func newSvc(repo Repository) *Service { return NewService(repo) }

func TestStockInValidatesProduct(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).StockIn(context.Background(), Movement{ProductID: uuid.Nil, Type: "IN", Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestStockInValidatesType(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).StockIn(context.Background(), Movement{ProductID: uuid.New(), Type: "SIDE", Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestStockInValidatesQuantity(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).StockIn(context.Background(), Movement{ProductID: uuid.New(), Type: "IN", Quantity: 0})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestStockInDelegates(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("StockIn", mock.Anything, mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == "IN" && mv.Quantity == 10
	})).Return(&Inventory{ProductID: pid, Quantity: 10}, nil)

	inv, err := newSvc(m).StockIn(context.Background(), Movement{ProductID: pid, Type: "IN", Quantity: 10})
	require.NoError(t, err)
	assert.Equal(t, 10, inv.Quantity)
}

func TestStockOutOverdrawConflict(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("StockOut", mock.Anything, mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == "OUT" && mv.Quantity == 50
	})).Return(nil, sharederr.ErrConflict)

	_, err := newSvc(m).StockOut(context.Background(), Movement{ProductID: pid, Type: "OUT", Quantity: 50})
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestStockOutDelegates(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("StockOut", mock.Anything, mock.Anything).Return(&Inventory{ProductID: pid, Quantity: 4}, nil)

	inv, err := newSvc(m).StockOut(context.Background(), Movement{ProductID: pid, Type: "OUT", Quantity: 6})
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
