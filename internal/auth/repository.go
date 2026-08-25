// GORM-backed implementation of the auth Repository interface.
package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory/internal/shared/dbutil"
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

func (r *GORMRepository) CreateUser(ctx context.Context, u *User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		if dbutil.IsUniqueViolation(err) {
			return ErrEmailTaken
		}
		return err
	}
	return nil
}

func (r *GORMRepository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *GORMRepository) FindUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *GORMRepository) UpdateUser(ctx context.Context, u *User) error {
	return r.db.WithContext(ctx).Save(u).Error
}

func (r *GORMRepository) FindRoleByName(ctx context.Context, name string) (*Role, error) {
	var role Role
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&role).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *GORMRepository) FindRoleByID(ctx context.Context, id uuid.UUID) (*Role, error) {
	var role Role
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&role).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}

// PermissionsByRoleID returns the permission codes granted to a role,
// ordered by code for a stable claim list. An empty result is valid — a role
// with no permissions is intentionally powerless.
func (r *GORMRepository) PermissionsByRoleID(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).Table("permissions").
		Select("permissions.code").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ?", roleID).
		Order("permissions.code ASC").
		Scan(&codes).Error
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *GORMRepository) CreateRefreshToken(ctx context.Context, t *RefreshToken) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *GORMRepository) FindRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error) {
	var t RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *GORMRepository) UpdateRefreshToken(ctx context.Context, t *RefreshToken) error {
	return r.db.WithContext(ctx).Save(t).Error
}

// RevokeFamily revokes every non-revoked refresh token in the family,
// killing all sessions derived from the same login.
func (r *GORMRepository) RevokeFamily(ctx context.Context, userID, familyID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&RefreshToken{}).
		Where("user_id = ? AND family_id = ? AND revoked_at IS NULL", userID, familyID).
		Update("revoked_at", time.Now()).Error
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

func (r *GORMRepository) CreateActivityLog(ctx context.Context, entry ActivityLogEntry) error {
	row := activityLogRow(entry)
	return r.db.WithContext(ctx).Create(&row).Error
}
