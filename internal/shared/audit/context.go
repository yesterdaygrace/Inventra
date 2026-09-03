package audit

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"inventory/internal/shared/middleware"
)

// EntryFromContext fills the request-derived fields (user, IP, user agent,
// request ID) of an audit Entry from the Gin context. Callers supply the
// domain fields (action, entity type/id, details, before/after data).
func EntryFromContext(c *gin.Context, e Entry) Entry {
	if uid := middleware.UserIDFromContext(c); uid != uuid.Nil {
		e.UserID = &uid
	}
	if ip := c.ClientIP(); ip != "" {
		e.IP = &ip
	}
	if ua := c.GetHeader("User-Agent"); ua != "" {
		e.UserAgent = &ua
	}
	if rid := c.GetString(middleware.RequestIDKey); rid != "" {
		e.RequestID = &rid
	}
	return e
}