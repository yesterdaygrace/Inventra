// GORM-backed implementation of the activitylog Repository interface.
package activitylog

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory/internal/shared/dbutil"
)

// Query filters and paginates the activity log listing.
type Query struct {
	UserID     uuid.UUID
	EntityType string
	EntityID   string
	Action     string
	From       *time.Time
	To         *time.Time
	Page       int
	PerPage    int
}

// GORMRepository persists activity logs using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// Create persists a single audit event.
func (r *GORMRepository) Create(l *ActivityLog) error {
	return r.db.Create(l).Error
}

// List returns a filtered, paginated page of audit events plus the total.
// Events are ordered newest-first. Every dynamic value is bound as a
// parameter so user input cannot be injected.
func (r *GORMRepository) List(q Query) ([]*ActivityLog, int64, error) {
	db := r.db.Model(&ActivityLog{}).Preload("User")

	if q.UserID != uuid.Nil {
		db = db.Where("user_id = ?", q.UserID)
	}
	if q.EntityType != "" {
		db = db.Where("LOWER(entity_type) = ?", strings.ToLower(q.EntityType))
	}
	if q.EntityID != "" {
		db = db.Where("entity_id = ?", q.EntityID)
	}
	if q.Action != "" {
		db = db.Where("LOWER(action) = ?", strings.ToLower(q.Action))
	}
	if q.From != nil {
		db = db.Where("created_at >= ?", *q.From)
	}
	if q.To != nil {
		db = db.Where("created_at <= ?", *q.To)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	p, per := dbutil.NormalizePage(q.Page, q.PerPage)
	var logs []*ActivityLog
	if err := db.Order("created_at DESC, id DESC").
		Offset((p - 1) * per).
		Limit(per).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
