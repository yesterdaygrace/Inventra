// Service logic for the activity log. Implements the audit.Recorder
// interface (failure-safe by design: recording errors never propagate).
package activitylog

import (
	"encoding/json"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"inventory/internal/shared/audit"
)

// Repository abstracts persistence for the activity log service.
type Repository interface {
	Create(l *ActivityLog) error
	List(q Query) ([]*ActivityLog, int64, error)
}

// Service records audit events and serves filtered reads.
type Service struct {
	repo Repository
	log  *zap.Logger
}

// NewService wires a repository and a logger into the service.
func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// Record persists an audit event. It never returns an error: any failure to
// write the log is itself logged and swallowed, so the caller's business
// operation is never failed by audit recording.
func (s *Service) Record(e audit.Entry) {
	var details *datatypes.JSON
	if e.Details != nil {
		raw, err := json.Marshal(e.Details)
		if err != nil {
			s.log.Warn("activitylog: marshal details", zap.Error(err))
			return
		}
		d := datatypes.JSON(raw)
		details = &d
	}

	l := &ActivityLog{
		UserID:     e.UserID,
		Action:     e.Action,
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		Details:    details,
		IP:         e.IP,
	}
	if err := s.repo.Create(l); err != nil {
		s.log.Warn("activitylog: record failed", zap.Error(err))
	}
}

// List returns a filtered, paginated page of audit events plus the total.
func (s *Service) List(q Query) ([]*ActivityLog, int64, error) {
	return s.repo.List(q)
}

// ParseUserID parses an optional user filter, returning the nil UUID when
// the raw value is empty so callers can distinguish "no filter" cleanly.
func ParseUserID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(raw)
}
