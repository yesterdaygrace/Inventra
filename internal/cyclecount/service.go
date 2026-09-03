// Service logic for cycle counting (PRD §24).
package cyclecount

import (
	"context"
	"time"

	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
)

// AdjustmentSubmitter is the adjustment-module capability counting needs to
// file variance requests. Implemented by *adjustment.Service.
type AdjustmentSubmitter interface {
	SubmitCountVariance(ctx context.Context, productID, warehouseID uuid.UUID, systemQuantity, countedQuantity int, requestedBy uuid.UUID) (adjustmentID uuid.UUID, err error)
}

// Service orchestrates count plans.
type Service struct {
	repo         *GORMRepository
	fileVariance AdjustmentSubmitter
	qtyReader    SystemQuantityReader
}

// NewService wires the repository, variance filer, and quantity reader.
func NewService(repo *GORMRepository, fileVariance AdjustmentSubmitter, qtyReader SystemQuantityReader) *Service {
	return &Service{repo: repo, fileVariance: fileVariance, qtyReader: qtyReader}
}

// CreatePlanInput is a validated plan request.
type CreatePlanInput struct {
	WarehouseID uuid.UUID
	Name        string
	ProductIDs  []uuid.UUID
	CreatedBy   uuid.UUID
}

// SystemQuantityReader snapshots on-hand quantities; implemented by the
// inventory repository in the composition root.
type SystemQuantityReader interface {
	SystemQuantity(ctx context.Context, productID, warehouseID uuid.UUID) (int, error)
}

// Service-level holder for the quantity reader (set via SetQuantityReader).
type qtyReaderHolder struct{ reader SystemQuantityReader }

// CreatePlan validates inputs and snapshots system quantities per SKU.
func (s *Service) CreatePlan(ctx context.Context, in CreatePlanInput) (*Plan, error) {
	if in.WarehouseID == uuid.Nil || in.Name == "" || len(in.ProductIDs) == 0 {
		return nil, sharederr.ErrValidation
	}
	seen := map[uuid.UUID]bool{}
	items := make([]Item, 0, len(in.ProductIDs))
	for _, pid := range in.ProductIDs {
		if pid == uuid.Nil || seen[pid] {
			return nil, sharederr.ErrValidation
		}
		seen[pid] = true
		qty, err := s.qtyReader.SystemQuantity(ctx, pid, in.WarehouseID)
		if err != nil {
			return nil, err
		}
		items = append(items, Item{
			ProductID:      pid,
			SystemQuantity: qty,
		})
	}

	plan := Plan{
		WarehouseID: in.WarehouseID,
		Name:        in.Name,
		Status:      PlanOpen,
		CreatedBy:   &in.CreatedBy,
	}
	if err := s.repo.CreatePlan(ctx, &plan, items); err != nil {
		return nil, err
	}
	return &plan, nil
}

// RecordCountInput is one Save & Next submission (PRD §56).
type RecordCountInput struct {
	ItemID          uuid.UUID
	CountedQuantity int
	CountedBy       uuid.UUID
}

// RecordCount stores a count for a pending item. A variance files an
// adjustment request (COUNT_VARIANCE) through the §23 workflow and links it
// to the item. Completing the last item completes the plan.
func (s *Service) RecordCount(ctx context.Context, in RecordCountInput) (*Item, error) {
	if in.CountedQuantity < 0 {
		return nil, sharederr.ErrValidation
	}

	it, err := s.repo.GetItem(ctx, in.ItemID)
	if err != nil {
		return nil, err
	}
	if it.CountedQuantity != nil {
		return nil, sharederr.ErrConflict
	}

	plan, err := s.repo.GetPlan(ctx, it.PlanID)
	if err != nil {
		return nil, err
	}
	if plan.Status != PlanOpen {
		return nil, sharederr.ErrConflict
	}

	now := time.Now()
	it.CountedQuantity = &in.CountedQuantity
	it.CountedBy = &in.CountedBy
	it.CountedAt = &now

	if in.CountedQuantity != it.SystemQuantity {
		adjID, err := s.fileVariance.SubmitCountVariance(
			ctx, it.ProductID, plan.WarehouseID,
			it.SystemQuantity, in.CountedQuantity, in.CountedBy)
		if err != nil {
			return nil, err
		}
		it.AdjustmentID = &adjID
	}

	if err := s.repo.SaveItem(ctx, it); err != nil {
		return nil, err
	}

	pending, err := s.repo.CountPendingItems(ctx, it.PlanID)
	if err != nil {
		return nil, err
	}
	if pending == 0 && plan.Status == PlanOpen {
		plan.Status = PlanCompleted
		if err := s.repo.SavePlan(ctx, plan); err != nil {
			return nil, err
		}
	}
	return it, nil
}

// ListPlans returns plans with progress.
func (s *Service) ListPlans(ctx context.Context) ([]*PlanView, int64, error) {
	return s.repo.ListPlans(ctx)
}

// PlanItems returns one plan's items.
func (s *Service) PlanItems(ctx context.Context, planID uuid.UUID) ([]*ItemView, error) {
	return s.repo.PlanItems(ctx, planID)
}
