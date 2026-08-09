// Package service — composes the stock summary report and exposes the
// cross-cutting repository contract used by the HTTP handlers.
package report

import (
	"context"
	"strconv"
)

// Repository is the data-access contract the report service relies on.
type Repository interface {
	StockSummary(ctx context.Context) (*StockSummary, error)
	CountProducts(ctx context.Context) (int64, error)
	InventoryValue(ctx context.Context) (float64, error)
}

// Service assembles the report read-model responses.
type Service struct {
	repo Repository
}

// NewService returns a report service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Summary builds the enriched stock summary: per-category rows, low-stock
// items, total product count, and total inventory valuation.
func (s *Service) Summary(ctx context.Context) (*StockSummary, error) {
	summary, err := s.repo.StockSummary(ctx)
	if err != nil {
		return nil, err
	}
	if summary.TotalProducts, err = s.repo.CountProducts(ctx); err != nil {
		return nil, err
	}
	if summary.TotalValue, err = s.repo.InventoryValue(ctx); err != nil {
		return nil, err
	}
	return summary, nil
}

// ExportRows flattens the summary into CSV header + data rows. Calling
// Summary first via the repository keeps the numbers identical between the
// JSON envelope and the downloadable file.
func (s *Service) ExportRows(ctx context.Context) ([]string, [][]string, error) {
	summary, err := s.Summary(ctx)
	if err != nil {
		return nil, nil, err
	}
	headers := []string{"category", "product_count", "total_qty", "total_value"}
	rows := make([][]string, 0, len(summary.Categories))
	for _, c := range summary.Categories {
		rows = append(rows, []string{
			c.Name,
			itoa(c.ProductCount),
			itoa(c.TotalQty),
			formatFloat(c.TotalValue),
		})
	}
	return headers, rows, nil
}

// LowStockRows flattens the low-stock items for the CSV export.
func (s *Service) LowStockRows(ctx context.Context) ([]string, [][]string, error) {
	summary, err := s.Summary(ctx)
	if err != nil {
		return nil, nil, err
	}
	headers := []string{"product_id", "sku", "name", "category", "quantity", "threshold", "value"}
	rows := make([][]string, 0, len(summary.LowStock))
	for _, it := range summary.LowStock {
		rows = append(rows, []string{
			it.ProductID,
			it.SKU,
			it.Name,
			it.Category,
			itoa(int64(it.Quantity)),
			itoa(int64(it.Threshold)),
			formatFloat(it.Value),
		})
	}
	return headers, rows, nil
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
