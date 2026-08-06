package dashboard

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

// mockRepo implements Repository for unit tests.
type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) CountProducts() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockRepo) CountCategories() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockRepo) InventoryValue() (float64, error) {
	args := m.Called()
	return args.Get(0).(float64), args.Error(1)
}

func (m *mockRepo) LowStockItems() ([]*LowStockItem, error) {
	args := m.Called()
	items, _ := args.Get(0).([]*LowStockItem)
	return items, args.Error(1)
}

func (m *mockRepo) RecentActivities(limit int) ([]*RecentActivity, error) {
	args := m.Called(limit)
	items, _ := args.Get(0).([]*RecentActivity)
	return items, args.Error(1)
}

func (m *mockRepo) Activities(page, perPage int) ([]*RecentActivity, int64, error) {
	args := m.Called(page, perPage)
	items, _ := args.Get(0).([]*RecentActivity)
	return items, args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) TopSellers(limit int) ([]*TopSeller, error) {
	args := m.Called(limit)
	items, _ := args.Get(0).([]*TopSeller)
	return items, args.Error(1)
}

func (m *mockRepo) InventoryMovement(since time.Time) ([]*DayMovement, error) {
	args := m.Called(since)
	items, _ := args.Get(0).([]*DayMovement)
	return items, args.Error(1)
}

func (m *mockRepo) CategoryDistribution() ([]*CategoryCount, error) {
	args := m.Called()
	items, _ := args.Get(0).([]*CategoryCount)
	return items, args.Error(1)
}

func (m *mockRepo) StockHealth() (StockHealth, error) {
	args := m.Called()
	return args.Get(0).(StockHealth), args.Error(1)
}

func (m *mockRepo) TotalQuantity() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func TestSummaryComposesAllCards(t *testing.T) {
	m := new(mockRepo)
	m.On("CountProducts").Return(int64(12), nil)
	m.On("CountCategories").Return(int64(4), nil)
	m.On("InventoryValue").Return(9876.50, nil)
	m.On("LowStockItems").Return([]*LowStockItem{{Name: "Widget"}}, nil)
	m.On("RecentActivities", RecentActivityLimit).Return([]*RecentActivity{{Action: "CREATE"}}, nil)
	m.On("StockHealth").Return(StockHealth{Healthy: 10, Low: 1, Critical: 1}, nil)

	sum, err := NewService(m).Summary()
	require.NoError(t, err)
	assert.Equal(t, int64(12), sum.TotalProducts)
	assert.Equal(t, int64(4), sum.TotalCategories)
	assert.Equal(t, 9876.50, sum.InventoryValue)
	assert.Equal(t, int64(1), sum.LowStockCount)
	assert.Equal(t, int64(1), sum.PendingRestock)
	assert.Equal(t, StockHealth{Healthy: 10, Low: 1, Critical: 1}, sum.WarehouseHealth)
	assert.Len(t, sum.RecentActivities, 1)
	assert.Len(t, sum.LowStockItems, 1)
}

func TestActivitiesReturnsPaginatedFeed(t *testing.T) {
	m := new(mockRepo)
	m.On("Activities", 1, 20).Return([]*RecentActivity{{Action: "CREATE"}}, int64(3), nil)
	items, total, err := NewService(m).Activities(1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, items, 1)
}

func TestSummaryPropagatesError(t *testing.T) {
	m := new(mockRepo)
	m.On("CountProducts").Return(int64(0), errBoom)
	_, err := NewService(m).Summary()
	assert.ErrorIs(t, err, errBoom)
}

func TestSummaryBubblesCategoryError(t *testing.T) {
	m := new(mockRepo)
	m.On("CountProducts").Return(int64(1), nil)
	m.On("CountCategories").Return(int64(0), errBoom)
	_, err := NewService(m).Summary()
	assert.ErrorIs(t, err, errBoom)
}

func TestInventoryMovementWindowsAndEndingBalance(t *testing.T) {
	m := new(mockRepo)
	now := time.Now().UTC()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -6)

	m.On("InventoryMovement", since).Return([]*DayMovement{
		{Day: since.Format("2006-01-02"), StockIn: 10, StockOut: 4},
	}, nil)
	m.On("TotalQuantity").Return(int64(100), nil)

	payload, err := NewService(m).InventoryMovement(7)
	require.NoError(t, err)
	require.Len(t, payload.Labels, 7)
	assert.Equal(t, int64(6), payload.Datasets[2].Data[0])   // net on first day = 10 - 4
	assert.Equal(t, int64(100), payload.Datasets[3].Data[6]) // ending series last element = live total
}

func TestInventoryMovementDefaultsDays(t *testing.T) {
	m := new(mockRepo)
	m.On("InventoryMovement", mock.AnythingOfType("time.Time")).Return(nil, nil)
	m.On("TotalQuantity").Return(int64(5), nil)

	payload, err := NewService(m).InventoryMovement(0)
	require.NoError(t, err)
	assert.Len(t, payload.Labels, DefaultMovementDays)
	assert.Equal(t, int64(5), payload.Datasets[3].Data[len(payload.Labels)-1])
}

func TestInventoryMovementCapsDays(t *testing.T) {
	m := new(mockRepo)
	m.On("InventoryMovement", mock.AnythingOfType("time.Time")).Return(nil, nil)
	m.On("TotalQuantity").Return(int64(1), nil)

	payload, err := NewService(m).InventoryMovement(MaxMovementDays + 10)
	require.NoError(t, err)
	assert.Len(t, payload.Labels, MaxMovementDays)
}

func TestCategoryDistribution(t *testing.T) {
	m := new(mockRepo)
	m.On("CategoryDistribution").Return([]*CategoryCount{
		{Name: "Books", Count: 5},
		{Name: "Electronics", Count: 3},
	}, nil)

	payload, err := NewService(m).CategoryDistribution()
	require.NoError(t, err)
	assert.Equal(t, []string{"Books", "Electronics"}, payload.Labels)
	require.Len(t, payload.Datasets, 1)
	assert.Equal(t, []int64{5, 3}, payload.Datasets[0].Data)
}

func TestTopSellingDefaultsAndClamps(t *testing.T) {
	m := new(mockRepo)
	m.On("TopSellers", 5).Return([]*TopSeller{{Name: "A", UnitsSold: 9}}, nil)
	payload, err := NewService(m).TopSelling(0)
	require.NoError(t, err)
	assert.Equal(t, []string{"A"}, payload.Labels)
	assert.Equal(t, []int64{9}, payload.Datasets[0].Data)
}
