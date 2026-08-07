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
	UserIDKey = "user_id"
	RoleKey   = "role"
)

// ClaimsParser extracts the authenticated identity from a raw bearer token.
// Implementations live in the auth module (which owns the token manager) so
// this package never imports auth, avoiding an import cycle.
type ClaimsParser interface {
	ParseAccessToken(raw string) (userID uuid.UUID, role string, err error)
}

// Auth requires a valid bearer access token. On success it stores userID
// and role in the context; otherwise it aborts with a 401 envelope.
func Auth(parser ClaimsParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		raw, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || raw == "" {
			response.Error(c, sharederr.ErrUnauthorized)
			c.Abort()
			return
		}

		userID, role, err := parser.ParseAccessToken(raw)
		if err != nil {
			response.Error(c, sharederr.ErrUnauthorized)
			c.Abort()
			return
		}
		c.Set(UserIDKey, userID)
		c.Set(RoleKey, role)
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
