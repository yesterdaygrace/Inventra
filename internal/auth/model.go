// Package auth provides authentication and authorization models.
package auth

import (
	"time"

	"github.com/google/uuid"
)

// Role represents a user role in the system (ADMIN or STAFF).
type Role struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string    `gorm:"type:text;unique;check:name IN ('ADMIN', 'STAFF')"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// User represents a system user account.
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name         string    `gorm:"type:text;not null"`
	Email        string    `gorm:"type:text;unique;not null"`
	PasswordHash string    `gorm:"type:text;not null"`
	RoleID       uuid.UUID `gorm:"type:uuid;not null"`
	Role         Role      `gorm:"foreignKey:RoleID"`
	IsActive     bool      `gorm:"not null;default:true"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}
