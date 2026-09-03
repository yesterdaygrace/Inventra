// Package errors defines the typed domain errors used across the
// application so handlers can map them to HTTP status codes uniformly.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel typed errors. Wrap with %w to preserve identity.
var (
	ErrNotFound        = errors.New("not found")
	ErrValidation      = errors.New("validation failed")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrConflict        = errors.New("conflict")
	ErrRateLimited     = errors.New("rate limit exceeded")
	ErrInternal        = errors.New("internal error")
	ErrDuplicateRequest = fmt.Errorf("%w: duplicate request", ErrConflict)
	ErrInsufficientStock = fmt.Errorf("%w: insufficient stock", ErrConflict)
)

// Is reports whether err matches any sentinel in targets.
func Is(err error, target error) bool {
	return errors.Is(err, target)
}
