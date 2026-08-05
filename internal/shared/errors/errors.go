// Package errors defines the typed domain errors used across the
// application so handlers can map them to HTTP status codes uniformly.
package errors

import "errors"

// Sentinel typed errors. Wrap with %w to preserve identity.
var (
	ErrNotFound     = errors.New("not found")
	ErrValidation   = errors.New("validation failed")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrInternal     = errors.New("internal error")
)

// Is reports whether err matches any sentinel in targets.
func Is(err error, target error) bool {
	return errors.Is(err, target)
}
