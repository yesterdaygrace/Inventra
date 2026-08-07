package validator

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

type sampleDTO struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
}

func TestValidateValid(t *testing.T) {
	v := New()
	err := v.Validate(sampleDTO{Name: "Ada", Email: "ada@example.com"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateInvalid(t *testing.T) {
	v := New()
	err := v.Validate(sampleDTO{Name: "", Email: "not-an-email"})
	if err == nil {
		t.Fatal("expected validation error for empty name + bad email")
	}
}

func TestValidateErrorWrapsOnce(t *testing.T) {
	v := New()
	err := v.Validate(sampleDTO{Name: "", Email: "bad"})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verr validator.ValidationErrors
	if !errors.As(err, &verr) {
		t.Fatalf("errors.As(err, &ValidationErrors) should match, got: %T", err)
	}

	msg := err.Error()
	if !strings.HasPrefix(msg, "validation failed:") {
		t.Errorf("expected message prefix 'validation failed:', got: %q", msg)
	}
	if got := strings.Count(msg, "sampleDTO"); got > 2 {
		t.Errorf("expected at most 2 mentions of the struct name, got %d in: %q", got, msg)
	}
}
