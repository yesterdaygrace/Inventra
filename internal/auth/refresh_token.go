// Package auth provides authentication and authorization models.
package auth

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents a refresh token for JWT token rotation.
type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	User      User      `gorm:"foreignKey:UserID"`
	TokenHash string    `gorm:"type:text;unique;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	RevokedAt *time.Time
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName overrides the default table name.
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
