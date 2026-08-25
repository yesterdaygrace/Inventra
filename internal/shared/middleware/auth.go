// JWT authentication middleware and RBAC helpers.
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/response"
)

// Context keys under which the middleware stores authenticated identity.
const (
	UserIDKey        = "user_id"
	RoleKey          = "role"
	PermissionsKey   = "permissions"
)

// ClaimsParser extracts the authenticated identity from a raw bearer token.
// Implementations live in the auth module (which owns the token manager) so
// this package never imports auth, avoiding an import cycle.
type ClaimsParser interface {
	ParseAccessToken(raw string) (userID uuid.UUID, role string, permissions []string, err error)
}

// Auth requires a valid bearer access token. On success it stores userID,
// role, and permissions in the context; otherwise it aborts with a 401
// envelope.
func Auth(parser ClaimsParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		raw, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || raw == "" {
			response.Error(c, sharederr.ErrUnauthorized)
			c.Abort()
			return
		}

		userID, role, permissions, err := parser.ParseAccessToken(raw)
		if err != nil {
			response.Error(c, sharederr.ErrUnauthorized)
			c.Abort()
			return
		}
		c.Set(UserIDKey, userID)
		c.Set(RoleKey, role)
		c.Set(PermissionsKey, permissions)
		c.Next()
	}
}

// RoleRequired rejects requests whose authenticated role is not in the
// allowed set (403). Must run after Auth.
func RoleRequired(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(RoleKey)
		for _, r := range allowed {
			if role == r {
				c.Next()
				return
			}
		}
		response.Error(c, sharederr.ErrForbidden)
		c.Abort()
	}
}

// Permission rejects requests whose authenticated claim-set lacks the
// required permission code (403). Must run after Auth. Empty or missing
// claim sets deny everything except the empty requirement.
func Permission(required string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms, _ := c.Get(PermissionsKey)
		list, _ := perms.([]string)
		for _, p := range list {
			if p == required {
				c.Next()
				return
			}
		}
		response.Error(c, sharederr.ErrForbidden)
		c.Abort()
	}
}

// PermissionsFromContext returns the authenticated user's permission codes,
// or an empty slice when absent.
func PermissionsFromContext(c *gin.Context) []string {
	val, ok := c.Get(PermissionsKey)
	if !ok {
		return nil
	}
	perms, ok := val.([]string)
	if !ok {
		return nil
	}
	return perms
}

// UserIDFromContext returns the authenticated user ID, or the zero value.
func UserIDFromContext(c *gin.Context) uuid.UUID {
	val, ok := c.Get(UserIDKey)
	if !ok {
		return uuid.Nil
	}
	id, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}

// RoleFromContext returns the authenticated role, or an empty string.
func RoleFromContext(c *gin.Context) string {
	val, ok := c.Get(RoleKey)
	if !ok {
		return ""
	}
	if role, ok := val.(string); ok {
		return role
	}
	return ""
}
