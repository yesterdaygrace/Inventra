// GORM-backed implementation of the auth Repository interface.
package auth

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	sharederr "inventory/internal/shared/errors"
)

// GORMRepository persists auth entities using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) CreateUser(u *User) error {
	if err := r.db.Create(u).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrEmailTaken
		}
		return err
	}
	return nil
}

func (r *GORMRepository) FindUserByEmail(email string) (*User, error) {
	var u User
	if err := r.db.Where("email = ?", email).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *GORMRepository) FindUserByID(id uuid.UUID) (*User, error) {
	var u User
	if err := r.db.Where("id = ?", id).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *GORMRepository) UpdateUser(u *User) error {
	return r.db.Save(u).Error
}

func (r *GORMRepository) FindRoleByName(name string) (*Role, error) {
	var role Role
	if err := r.db.Where("name = ?", name).First(&role).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *GORMRepository) FindRoleByID(id uuid.UUID) (*Role, error) {
	var role Role
	if err := r.db.Where("id = ?", id).First(&role).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *GORMRepository) CreateRefreshToken(t *RefreshToken) error {
	return r.db.Create(t).Error
}

func (r *GORMRepository) FindRefreshTokenByHash(hash string) (*RefreshToken, error) {
	var t RefreshToken
	if err := r.db.Where("token_hash = ?", hash).First(&t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *GORMRepository) UpdateRefreshToken(t *RefreshToken) error {
	return r.db.Save(t).Error
}

// activityLogRow mirrors the activity_logs table columns needed by auth.
// It lives here so the auth package need not import activitylog (which
// imports auth.User — a cycle); it writes the same physical tables.
type activityLogRow struct {
	UserID     *uuid.UUID `gorm:"column:user_id"`
	Action     string     `gorm:"column:action"`
	EntityType string     `gorm:"column:entity_type"`
}

func (activityLogRow) TableName() string { return "activity_logs" }

// isUniqueViolation detects a PostgreSQL unique-constraint violation so
// create/update callers can translate it into ErrConflict.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}

func (r *GORMRepository) CreateActivityLog(entry ActivityLogEntry) error {
	row := activityLogRow{
		UserID:     entry.UserID,
		Action:     entry.Action,
		EntityType: entry.EntityType,
	}
	return r.db.Create(&row).Error
}
