// Package activitylog provides audit-trail tracking of user actions.
package activitylog

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"inventory/internal/auth"
)

type ActivityLog struct {
	ID         uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     *uuid.UUID      `gorm:"type:uuid"`
	User       *auth.User      `gorm:"foreignKey:UserID"`
	Action     string          `gorm:"type:text;not null"`
	EntityType string          `gorm:"type:text;not null"`
	EntityID   *string         `gorm:"type:text"`
	Details    *datatypes.JSON `gorm:"type:jsonb"`
	IP         *string         `gorm:"type:text"`
	CreatedAt  time.Time       `gorm:"autoCreateTime"`
}

// TableName overrides the default table name.
func (ActivityLog) TableName() string {
	return "activity_logs"
}
