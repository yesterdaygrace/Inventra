// Package user provides admin management of system user accounts.
// It reuses the auth domain models (auth.User, auth.Role) and adds
// list/search/paginate, assign-role, and activate/deactivate flows.
package user

import "inventory/internal/auth"

// User and Role are the auth models reused unchanged by this module.
type (
	User = auth.User
	Role = auth.Role
)

// Query carries the list filters and pagination for List.
type Query struct {
	Name     string // substring match on name (case-insensitive)
	Email    string // substring match on email (case-insensitive)
	Role     string // exact role name (empty = any)
	IsActive *bool  // nil = any
	Page     int
	PerPage  int
}
