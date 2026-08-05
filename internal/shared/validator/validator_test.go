package validator

import "testing"

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
