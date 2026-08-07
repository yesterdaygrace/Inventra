// Package validator wraps go-playground/validator/v10 so all DTO
// validation flows through one constructor and one error format.
package validator

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// Validator is the shared struct validator.
type Validator struct {
	validate *validator.Validate
}

// New creates a Validator.
func New() *Validator {
	return &Validator{validate: validator.New()}
}

// Validate checks s (a struct or slice of structs) and returns a
// human-readable validation error, or nil when valid.
func (v *Validator) Validate(s any) error {
	if err := v.validate.Struct(s); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}
