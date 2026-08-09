package report

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) StockSummary(ctx context.Context) (*StockSummary, error) {
	args := m.Called(ctx)
	summary, _ := args.Get(0).(*StockSummary)
	return summary, args.Error(1)
}

func (m *mockRepo) CountProducts(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockRepo) InventoryValue(ctx context.Context) (float64, error) {
	args := m.Called(ctx)
	return args.Get(0).(float64), args.Error(1)
}

func TestSummaryComposesEnvelope(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(&StockSummary{
		Categories: []*CategorySummary{{Name: "Books", ProductCount: 5, TotalQty: 12, TotalValue: 300}},
		LowStock:   []*LowStockItem{{Name: "Widget"}},
	}, nil)
	m.On("CountProducts", mock.Anything).Return(int64(9), nil)
	m.On("InventoryValue", mock.Anything).Return(5432.10, nil)

	sum, err := NewService(m).Summary(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(9), sum.TotalProducts)
	assert.Equal(t, 5432.10, sum.TotalValue)
	assert.Len(t, sum.Categories, 1)
	assert.Len(t, sum.LowStock, 1)
}

func TestSummaryPropagatesError(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(nil, errBoom)
	_, err := NewService(m).Summary(context.Background())
	assert.ErrorIs(t, err, errBoom)
}

func TestSummaryBubblesCountError(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(&StockSummary{Categories: []*CategorySummary{}}, nil)
	m.On("CountProducts", mock.Anything).Return(int64(0), errBoom)
	_, err := NewService(m).Summary(context.Background())
	assert.ErrorIs(t, err, errBoom)
}

func TestSummaryBubblesValueError(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(&StockSummary{Categories: []*CategorySummary{}}, nil)
	m.On("CountProducts", mock.Anything).Return(int64(1), nil)
	m.On("InventoryValue", mock.Anything).Return(0.0, errBoom)
	_, err := NewService(m).Summary(context.Background())
	assert.ErrorIs(t, err, errBoom)
}

func TestExportRowsFlattensCategories(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(&StockSummary{
		Categories: []*CategorySummary{
			{Name: "Books", ProductCount: 2, TotalQty: 10, TotalValue: 250.5},
		},
		LowStock: []*LowStockItem{},
	}, nil)
	m.On("CountProducts", mock.Anything).Return(int64(2), nil)
	m.On("InventoryValue", mock.Anything).Return(250.50, nil)

	headers, rows, err := NewService(m).ExportRows(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"category", "product_count", "total_qty", "total_value"}, headers)
	require.Len(t, rows, 1)
	assert.Equal(t, []string{"Books", "2", "10", "250.50"}, rows[0])
}

func TestLowStockRowsFlattensItems(t *testing.T) {
	m := new(mockRepo)
	m.On("StockSummary", mock.Anything).Return(&StockSummary{
		Categories: []*CategorySummary{},
		LowStock: []*LowStockItem{
			{ProductID: "pid-1", SKU: "SK", Name: "Widget", Category: "Tools", Quantity: 2, Threshold: 5, Value: 20},
		},
	}, nil)
	m.On("CountProducts", mock.Anything).Return(int64(1), nil)
	m.On("InventoryValue", mock.Anything).Return(20.0, nil)

	headers, rows, err := NewService(m).LowStockRows(context.Background())
	require.NoError(t, err)
	assert.Len(t, headers, 7)
	require.Len(t, rows, 1)
	assert.Equal(t, []string{"pid-1", "SK", "Widget", "Tools", "2", "5", "20.00"}, rows[0])
}
