// Service logic for dashboard aggregates. Consumes a Repository interface
// and composes summary cards plus Recharts-shaped chart payloads.
package dashboard

import (
	"time"
)

// RecentActivityLimit is the number of recent audit events in the summary.
const RecentActivityLimit = 10

// DefaultMovementDays is the chart window when the caller omits `days`.
const DefaultMovementDays = 30

// MaxMovementDays caps the inventory-movement window.
const MaxMovementDays = 365

// Repository abstracts the aggregation queries behind the dashboard service.
type Repository interface {
	CountProducts() (int64, error)
	CountCategories() (int64, error)
	InventoryValue() (float64, error)
	LowStockItems() ([]*LowStockItem, error)
	RecentActivities(limit int) ([]*RecentActivity, error)
	TopSellers(limit int) ([]*TopSeller, error)
	InventoryMovement(since time.Time) ([]*DayMovement, error)
	CategoryDistribution() ([]*CategoryCount, error)
	StockHealth() (StockHealth, error)
	TotalQuantity() (int64, error)
}

// Service composes dashboard aggregates into response payloads.
type Service struct {
	repo Repository
}

// NewService wires a repository into the service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Summary is the aggregate payload for the GET /dashboard/summary card set.
type Summary struct {
	TotalProducts    int64             `json:"total_products"`
	TotalCategories  int64             `json:"total_categories"`
	InventoryValue   float64           `json:"inventory_value"`
	LowStockCount    int64             `json:"low_stock_count"`
	PendingRestock   int64             `json:"pending_restock"`
	WarehouseHealth  StockHealth       `json:"warehouse_health"`
	RecentActivities []*RecentActivity `json:"recent_activities"`
	LowStockItems    []*LowStockItem   `json:"low_stock_items"`
}

// ChartPayload is the Recharts contract: aligned label/data series.
type ChartPayload struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
}

// ChartDataset is one named series aligned to ChartPayload.Labels.
type ChartDataset struct {
	Label string  `json:"label"`
	Data  []int64 `json:"data"`
}

// Summary fetches every KPI card plus the dashboard widgets in one pass.
func (s *Service) Summary() (*Summary, error) {
	products, err := s.repo.CountProducts()
	if err != nil {
		return nil, err
	}
	categories, err := s.repo.CountCategories()
	if err != nil {
		return nil, err
	}
	value, err := s.repo.InventoryValue()
	if err != nil {
		return nil, err
	}
	lowStock, err := s.repo.LowStockItems()
	if err != nil {
		return nil, err
	}
	recent, err := s.repo.RecentActivities(RecentActivityLimit)
	if err != nil {
		return nil, err
	}
	health, err := s.repo.StockHealth()
	if err != nil {
		return nil, err
	}

	return &Summary{
		TotalProducts:    products,
		TotalCategories:  categories,
		InventoryValue:   value,
		LowStockCount:    int64(len(lowStock)),
		PendingRestock:   int64(len(lowStock)),
		WarehouseHealth:  health,
		RecentActivities: recent,
		LowStockItems:    lowStock,
	}, nil
}

// InventoryMovement returns per-day in/out/net/ending series for the last
// `days` calendar days. Ending balances walk backward from the current total
// quantity, so the most recent day always equals the live stock count.
func (s *Service) InventoryMovement(days int) (*ChartPayload, error) {
	if days < 1 {
		days = DefaultMovementDays
	}
	if days > MaxMovementDays {
		days = MaxMovementDays
	}

	now := time.Now().UTC()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, -(days - 1))

	moves, err := s.repo.InventoryMovement(since)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.TotalQuantity()
	if err != nil {
		return nil, err
	}

	byDay := make(map[string]DayMovement, len(moves))
	for _, m := range moves {
		byDay[m.Day] = *m
	}

	labels := make([]string, days)
	stockIn := make([]int64, days)
	stockOut := make([]int64, days)
	net := make([]int64, days)
	ending := make([]int64, days)

	for i := 0; i < days; i++ {
		day := since.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		labels[i] = key
		m, ok := byDay[key]
		if ok {
			stockIn[i] = int64(m.StockIn)
			stockOut[i] = int64(m.StockOut)
		}
		net[i] = stockIn[i] - stockOut[i]
	}

	ending[days-1] = total
	for i := days - 2; i >= 0; i-- {
		ending[i] = ending[i+1] - net[i+1]
	}

	return &ChartPayload{
		Labels: labels,
		Datasets: []ChartDataset{
			{Label: "Stock In", Data: stockIn},
			{Label: "Stock Out", Data: stockOut},
			{Label: "Net", Data: net},
			{Label: "Ending", Data: ending},
		},
	}, nil
}

// CategoryDistribution returns product counts per category as a bar payload.
func (s *Service) CategoryDistribution() (*ChartPayload, error) {
	counts, err := s.repo.CategoryDistribution()
	if err != nil {
		return nil, err
	}

	labels := make([]string, 0, len(counts))
	data := make([]int64, 0, len(counts))
	for _, c := range counts {
		labels = append(labels, c.Name)
		data = append(data, c.Count)
	}

	return &ChartPayload{
		Labels: labels,
		Datasets: []ChartDataset{
			{Label: "Products", Data: data},
		},
	}, nil
}

// TopSelling returns the top `limit` products by units sold as a bar payload.
func (s *Service) TopSelling(limit int) (*ChartPayload, error) {
	if limit < 1 || limit > 50 {
		limit = 5
	}
	sellers, err := s.repo.TopSellers(limit)
	if err != nil {
		return nil, err
	}

	labels := make([]string, 0, len(sellers))
	data := make([]int64, 0, len(sellers))
	for _, s := range sellers {
		labels = append(labels, s.Name)
		data = append(data, int64(s.UnitsSold))
	}

	return &ChartPayload{
		Labels: labels,
		Datasets: []ChartDataset{
			{Label: "Units Sold", Data: data},
		},
	}, nil
}
