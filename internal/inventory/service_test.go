package inventory

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

func (m *mockRepo) StockIn(mv Movement) (*Inventory, error) {
	args := m.Called(mv)
	if inv, ok := args.Get(0).(*Inventory); ok {
		return inv, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) StockOut(mv Movement) (*Inventory, error) {
	args := m.Called(mv)
	if inv, ok := args.Get(0).(*Inventory); ok {
		return inv, args.Error(1)
	}
	return nil, args.Error(1)
}

func newSvc(repo Repository) *Service { return NewService(repo) }

func TestStockInValidatesProduct(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).StockIn(Movement{ProductID: uuid.Nil, Type: "IN", Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestStockInValidatesType(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).StockIn(Movement{ProductID: uuid.New(), Type: "SIDE", Quantity: 5})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestStockInValidatesQuantity(t *testing.T) {
	m := new(mockRepo)
	_, err := newSvc(m).StockIn(Movement{ProductID: uuid.New(), Type: "IN", Quantity: 0})
	assert.ErrorIs(t, err, sharederr.ErrValidation)
}

func TestStockInDelegates(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("StockIn", mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == "IN" && mv.Quantity == 10
	})).Return(&Inventory{ProductID: pid, Quantity: 10}, nil)

	inv, err := newSvc(m).StockIn(Movement{ProductID: pid, Type: "IN", Quantity: 10})
	require.NoError(t, err)
	assert.Equal(t, 10, inv.Quantity)
}

func TestStockOutOverdrawConflict(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("StockOut", mock.MatchedBy(func(mv Movement) bool {
		return mv.ProductID == pid && mv.Type == "OUT" && mv.Quantity == 50
	})).Return(nil, sharederr.ErrConflict)

	_, err := newSvc(m).StockOut(Movement{ProductID: pid, Type: "OUT", Quantity: 50})
	assert.ErrorIs(t, err, sharederr.ErrConflict)
}

func TestStockOutDelegates(t *testing.T) {
	m := new(mockRepo)
	pid := uuid.New()
	m.On("StockOut", mock.Anything).Return(&Inventory{ProductID: pid, Quantity: 4}, nil)

	inv, err := newSvc(m).StockOut(Movement{ProductID: pid, Type: "OUT", Quantity: 6})
	require.NoError(t, err)
	assert.Equal(t, 4, inv.Quantity)
}

var _ Repository = (*mockRepo)(nil)
