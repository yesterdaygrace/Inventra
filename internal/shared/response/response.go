// Package response provides the unified JSON envelope for all API
// responses, per PRD §44/§45:
//
//	success: {"data": ..., "meta": {"page","per_page","total","total_pages"}}
//	error:   {"error": {"code": "STABLE_CODE", "message": "..."}}
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"inventory/internal/shared/errors"
)

// Pagination describes the paging metadata carried in `meta`.
type Pagination struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// ErrorBody is the stable machine-readable error payload (PRD §67 codes).
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Body is the standard response envelope. Exactly one of Data or Error is
// populated; Meta accompanies Data on paginated lists.
type Body struct {
	Data  any         `json:"data,omitempty"`
	Meta  *Pagination `json:"meta,omitempty"`
	Error *ErrorBody  `json:"error,omitempty"`
}

// OK writes a 200 envelope with data.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Data: data})
}

// Created writes a 201 envelope with data.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Body{Data: data})
}

// Message writes a 200 envelope carrying an informational payload.
func Message(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Body{Data: gin.H{"message": msg}})
}

// Error writes the error envelope and chooses the status code and stable
// machine-readable code from the typed error. Unknown errors map to 500.
func Error(c *gin.Context, err error) {
	status, code, msg := statusFor(err)
	c.JSON(status, Body{Error: &ErrorBody{Code: code, Message: msg}})
}

// codeFor maps a typed error onto a stable machine-readable code (PRD §67).
func codeFor(err error) string {
	switch {
	case errors.Is(err, errors.ErrValidation):
		return "VALIDATION_FAILED"
	case errors.Is(err, errors.ErrUnauthorized):
		return "UNAUTHORIZED"
	case errors.Is(err, errors.ErrForbidden):
		return "FORBIDDEN"
	case errors.Is(err, errors.ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, errors.ErrDuplicateRequest):
		return "DUPLICATE_REQUEST"
	case errors.Is(err, errors.ErrInsufficientStock):
		return "INSUFFICIENT_STOCK"
	case errors.Is(err, errors.ErrConflict):
		return "CONFLICT"
	case errors.Is(err, errors.ErrRateLimited):
		return "RATE_LIMITED"
	default:
		return "INTERNAL_ERROR"
	}
}

func statusFor(err error) (int, string, string) {
	code := codeFor(err)
	switch {
	case errors.Is(err, errors.ErrValidation):
		return http.StatusBadRequest, code, err.Error()
	case errors.Is(err, errors.ErrUnauthorized):
		return http.StatusUnauthorized, code, "unauthorized"
	case errors.Is(err, errors.ErrForbidden):
		return http.StatusForbidden, code, "forbidden"
	case errors.Is(err, errors.ErrNotFound):
		return http.StatusNotFound, code, err.Error()
	case errors.Is(err, errors.ErrConflict), errors.Is(err, errors.ErrDuplicateRequest), errors.Is(err, errors.ErrInsufficientStock):
		return http.StatusConflict, code, err.Error()
	case errors.Is(err, errors.ErrRateLimited):
		return http.StatusTooManyRequests, code, err.Error()
	default:
		return http.StatusInternalServerError, code, "internal server error"
	}
}

// Paginated writes a 200 envelope with data and paging metadata in `meta`.
func Paginated(c *gin.Context, data any, p *Pagination) {
	c.JSON(http.StatusOK, Body{Data: data, Meta: p})
}
