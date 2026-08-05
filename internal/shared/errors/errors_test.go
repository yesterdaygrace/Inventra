package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelIdentity(t *testing.T) {
	err := fmt.Errorf("product 1: %w", ErrNotFound)
	if !errors.Is(err, ErrNotFound) {
		t.Error("wrapped ErrNotFound should match via errors.Is")
	}
	if Is(err, ErrNotFound) != true {
		t.Error("Is(err, ErrNotFound) should be true")
	}
	if Is(err, ErrConflict) {
		t.Error("Is(err, ErrConflict) should be false")
	}
}

func TestSentinelsDistinct(t *testing.T) {
	all := []error{ErrNotFound, ErrValidation, ErrUnauthorized, ErrForbidden, ErrConflict, ErrInternal}
	seen := map[string]bool{}
	for _, e := range all {
		if seen[e.Error()] {
			t.Errorf("duplicate error message: %q", e.Error())
		}
		seen[e.Error()] = true
	}
}
