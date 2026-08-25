// Service logic for the adjustment approval workflow (PRD §23).
package adjustment

import (
	"context"
	"time"

	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
)

// Service orchestrates adjustment submission and review.
type Service struct {
	repo *GORMRepository
	// apply is wired to inventory.Service.ApplyCorrection via an adapter in
	// the composition root, keeping this module decoupled from inventory
	// types beyond the UUID contract.
	apply func(ctx context.Context, productID, warehouseID uuid.UUID, targetQuantity int, referenceType, referenceID, reason string, userID *uuid.UUID) error
}

// NewService wires the repository and the stock applier.
func NewService(repo *GORMRepository, apply func(ctx context.Context, productID, warehouseID uuid.UUID, targetQuantity int, referenceType, referenceID, reason string, userID *uuid.UUID) error) *Service {
	return &Service{repo: repo, apply: apply}
}

// SubmitInput is a validated adjustment request.
type SubmitInput struct {
	ProductID       uuid.UUID
	WarehouseID     uuid.UUID
	CountedQuantity int
	Reason          string
	Note            *string
	RequestedBy     uuid.UUID
	// AutoApprove is true when the performer holds inventory.adjust; the
	// value threshold still gates it.
	AutoApprove bool
}

// Submit snapshots the system quantity, prices the delta, and either
// auto-applies (performer may adjust AND |value| < threshold) or queues the
// request as PENDING for a manager.
func (s *Service) Submit(ctx context.Context, in SubmitInput) (*Adjustment, error) {
	if in.ProductID == uuid.Nil || in.WarehouseID == uuid.Nil {
		return nil, sharederr.ErrValidation
	}
	if in.CountedQuantity < 0 {
		return nil, sharederr.ErrValidation
	}
	if !reasonSet[in.Reason] {
		return nil, sharederr.ErrValidation
	}

	systemQty, err := s.repo.SystemQuantity(ctx, in.ProductID, in.WarehouseID)
	if err != nil {
		return nil, err
	}
	cost, err := s.repo.LastKnownCost(ctx, in.ProductID, in.WarehouseID)
	if err != nil {
		return nil, err
	}

	delta := in.CountedQuantity - systemQty
	if delta < 0 {
		delta = -delta
	}
	value := float64(delta) * cost

	a := Adjustment{
		ProductID:       in.ProductID,
		WarehouseID:     in.WarehouseID,
		SystemQuantity:  systemQty,
		CountedQuantity: in.CountedQuantity,
		Reason:          in.Reason,
		Note:            in.Note,
		Status:          StatusPending,
		RequestedBy:     &in.RequestedBy,
	}

	threshold, err := s.repo.Threshold(ctx)
	if err != nil {
		return nil, err
	}
	if in.AutoApprove && value < threshold {
		// Apply immediately inside the submit call.
		if err := s.apply(ctx, in.ProductID, in.WarehouseID, in.CountedQuantity,
			"adjustment", "", in.Reason, &in.RequestedBy); err != nil {
			return nil, err
		}
		now := time.Now()
		a.Status = StatusApproved
		a.ReviewedBy = &in.RequestedBy
		a.ReviewedAt = &now
		a.AppliedValue = &value
	}
	if err := s.repo.Create(ctx, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Approve applies a PENDING adjustment through the inventory module.
func (s *Service) Approve(ctx context.Context, id, reviewer uuid.UUID) (*Adjustment, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Status != StatusPending {
		return nil, sharederr.ErrConflict
	}

	cost, err := s.repo.LastKnownCost(ctx, a.ProductID, a.WarehouseID)
	if err != nil {
		return nil, err
	}
	delta := a.CountedQuantity - a.SystemQuantity
	if delta < 0 {
		delta = -delta
	}
	value := float64(delta) * cost

	if err := s.apply(ctx, a.ProductID, a.WarehouseID, a.CountedQuantity,
		"adjustment", a.ID.String(), a.Reason, &reviewer); err != nil {
		return nil, err
	}

	now := time.Now()
	a.Status = StatusApproved
	a.ReviewedBy = &reviewer
	a.ReviewedAt = &now
	a.AppliedValue = &value
	if err := s.repo.Save(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Reject declines a PENDING adjustment without touching stock.
func (s *Service) Reject(ctx context.Context, id, reviewer uuid.UUID) (*Adjustment, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Status != StatusPending {
		return nil, sharederr.ErrConflict
	}
	now := time.Now()
	a.Status = StatusRejected
	a.ReviewedBy = &reviewer
	a.ReviewedAt = &now
	if err := s.repo.Save(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// List returns filtered, paginated adjustments.
func (s *Service) List(ctx context.Context, q ListQuery) ([]*AdjustmentView, int64, error) {
	return s.repo.List(ctx, q)
}

// SubmitCountVariance files a COUNT_VARIANCE adjustment on behalf of the
// counting workflow. Variances always queue for manager review.
func (s *Service) SubmitCountVariance(ctx context.Context, productID, warehouseID uuid.UUID, systemQuantity, countedQuantity int, requestedBy uuid.UUID) (uuid.UUID, error) {
	a, err := s.Submit(ctx, SubmitInput{
		ProductID:       productID,
		WarehouseID:     warehouseID,
		CountedQuantity: countedQuantity,
		Reason:          ReasonCountVariance,
		RequestedBy:     requestedBy,
		AutoApprove:     false,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return a.ID, nil
}
