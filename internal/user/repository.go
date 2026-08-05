// GORM-backed implementation of the user Repository interface.
package user

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	sharederr "inventory/internal/shared/errors"
)

// GORMRepository persists users and roles using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// List returns a page of users matching the filters plus the total count
// of matches before paging.
func (r *GORMRepository) List(q Query) ([]*User, int64, error) {
	db := r.db.Model(&User{})

	if q.Name != "" {
		db = db.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(q.Name)+"%")
	}
	if q.Email != "" {
		db = db.Where("LOWER(email) LIKE ?", "%"+strings.ToLower(q.Email)+"%")
	}
	if q.Role != "" {
		db = db.Where("role_id IN (SELECT id FROM roles WHERE name = ?)", q.Role)
	}
	if q.IsActive != nil {
		db = db.Where("is_active = ?", *q.IsActive)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.PerPage < 1 || q.PerPage > 100 {
		q.PerPage = 20
	}

	var users []*User
	if err := db.Preload("Role").
		Order("created_at DESC").
		Offset((q.Page - 1) * q.PerPage).
		Limit(q.PerPage).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// FindByID returns a user by primary key.
func (r *GORMRepository) FindByID(id uuid.UUID) (*User, error) {
	var u User
	if err := r.db.Preload("Role").Where("id = ?", id).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// FindByEmail returns a user by exact email address.
func (r *GORMRepository) FindByEmail(email string) (*User, error) {
	var u User
	if err := r.db.Where("email = ?", email).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// Update persists user changes.
func (r *GORMRepository) Update(u *User) error {
	return r.db.Save(u).Error
}

// FindRoleByName returns a role by exact name.
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

// CountAdmins returns the number of ADMIN users.
func (r *GORMRepository) CountAdmins() (int64, error) {
	var count int64
	err := r.db.Model(&User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("roles.name = ? AND users.is_active = ?", "ADMIN", true).
		Count(&count).Error
	return count, err
}
