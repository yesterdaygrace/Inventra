package dbutil

import (
	"errors"
	"testing"
)

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain", errors.New("boom"), false},
		{"unique", errors.New(`duplicate key value violates unique constraint "uq"`), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUniqueViolation(tt.err); got != tt.want {
				t.Errorf("IsUniqueViolation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsForeignKeyViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain", errors.New("boom"), false},
		{"fk", errors.New(`violates foreign key constraint "fk_products"`), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsForeignKeyViolation(tt.err); got != tt.want {
				t.Errorf("IsForeignKeyViolation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizePage(t *testing.T) {
	tests := []struct {
		page, per, wantPage, wantPer int
	}{
		{1, 20, 1, 20},
		{0, 0, 1, 20},
		{-1, 200, 1, 20},
		{5, 50, 5, 50},
	}
	for _, tt := range tests {
		gotP, gotPer := NormalizePage(tt.page, tt.per)
		if gotP != tt.wantPage || gotPer != tt.wantPer {
			t.Errorf("NormalizePage(%d,%d) = (%d,%d), want (%d,%d)",
				tt.page, tt.per, gotP, gotPer, tt.wantPage, tt.wantPer)
		}
	}
}
