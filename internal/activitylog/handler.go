// HTTP handlers for activity logs. The listing route is admin-only and
// returns the shared response envelope with pagination.
package activitylog

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/response"
	"inventory/internal/shared/validator"
)

// Handler exposes the activity log routes.
type Handler struct {
	svc *Service
	val *validator.Validator
}

// NewHandler wires the service and validator.
func NewHandler(svc *Service, val *validator.Validator) *Handler {
	return &Handler{svc: svc, val: val}
}

type listRequest struct {
	UserID     string `form:"user_id"`
	EntityType string `form:"entity_type"`
	EntityID   string `form:"entity_id"`
	Action     string `form:"action"`
	From       string `form:"from"`
	To         string `form:"to"`
	Page       int    `form:"page"`
	PerPage    int    `form:"per_page"`
}

type activityEnvelope struct {
	ID         uuid.UUID  `json:"id"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	UserName   string     `json:"user_name,omitempty"`
	Action     string     `json:"action"`
	EntityType string     `json:"entity_type"`
	EntityID   *string    `json:"entity_id,omitempty"`
	Details    any        `json:"details,omitempty"`
	IP         *string    `json:"ip,omitempty"`
	CreatedAt  string     `json:"created_at"`
}

func activityResponse(l *ActivityLog) activityEnvelope {
	env := activityEnvelope{
		ID:         l.ID,
		UserID:     l.UserID,
		Action:     l.Action,
		EntityType: l.EntityType,
		EntityID:   l.EntityID,
		IP:         l.IP,
		CreatedAt:  l.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if l.User != nil {
		env.UserName = l.User.Name
	}
	if l.Details != nil {
		var details any
		if err := json.Unmarshal(*l.Details, &details); err == nil {
			env.Details = details
		}
	}
	return env
}

// List handles GET /activity-logs (admin).
func (h *Handler) List(c *gin.Context) {
	var req listRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	uid, err := ParseUserID(req.UserID)
	if err != nil {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	var from, to *time.Time
	if req.From != "" {
		t, err := time.Parse(time.RFC3339, req.From)
		if err != nil {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		from = &t
	}
	if req.To != "" {
		t, err := time.Parse(time.RFC3339, req.To)
		if err != nil {
			response.Error(c, sharederr.ErrValidation)
			return
		}
		to = &t
	}
	if from != nil && to != nil && from.After(*to) {
		response.Error(c, sharederr.ErrValidation)
		return
	}

	logs, total, err := h.svc.List(Query{
		UserID:     uid,
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		Action:     req.Action,
		From:       from,
		To:         to,
		Page:       req.Page,
		PerPage:    req.PerPage,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]activityEnvelope, 0, len(logs))
	for _, l := range logs {
		items = append(items, activityResponse(l))
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	per := req.PerPage
	if per < 1 {
		per = 20
	}
	response.Paginated(c, items, &response.Pagination{
		Page:       page,
		PerPage:    per,
		Total:      total,
		TotalPages: int((total + int64(per) - 1) / int64(per)),
	})
}
