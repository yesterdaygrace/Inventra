// Package response provides the unified JSON envelope for all API
// responses: {success, message, data, pagination}.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"inventory/internal/shared/errors"
)

// Pagination describes the paging metadata in list responses.
type Pagination struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// Body is the standard response envelope.
type Body struct {
	Success    bool        `json:"success"`
	Message    string      `json:"message,omitempty"`
	Data       any         `json:"data,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// OK writes a 200 envelope with data.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Success: true, Data: data})
}

// Created writes a 201 envelope with data.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Body{Success: true, Data: data})
}

// Message writes a 200 envelope carrying only a message.
func Message(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Body{Success: true, Message: msg})
}

// Error writes an error envelope and chooses the status code from the
// typed error. Unknown errors map to 500.
func Error(c *gin.Context, err error) {
	status, msg := statusFor(err)
	c.JSON(status, Body{Success: false, Message: msg})
}

func statusFor(err error) (int, string) {
	switch {
	case errors.Is(err, errors.ErrValidation):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, errors.ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, errors.ErrForbidden):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, errors.ErrNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, errors.ErrConflict):
		return http.StatusConflict, err.Error()
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}

// Paginated writes a 200 envelope with data and pagination metadata.
func Paginated(c *gin.Context, data any, p *Pagination) {
	c.JSON(http.StatusOK, Body{Success: true, Data: data, Pagination: p})
}
