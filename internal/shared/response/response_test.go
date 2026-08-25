package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	sharederrors "inventory/internal/shared/errors"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// decodeErr pulls the error payload out of a raw JSON envelope.
func decodeErr(t *testing.T, raw string) ErrorBody {
	t.Helper()
	var wire struct {
		Error *ErrorBody `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if wire.Error == nil {
		t.Fatalf("error object missing in %s", raw)
	}
	return *wire.Error
}

func TestOKEnvelopeShape(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	OK(c, gin.H{"id": 1})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, ok := body.Data.(map[string]any)
	if !ok || data["id"] != float64(1) {
		t.Errorf("data mismatch: %#v", body.Data)
	}
	if body.Meta != nil {
		t.Error("meta should be nil for OK")
	}
	if body.Error != nil {
		t.Error("error should be nil for OK")
	}
	// The success flag is gone; data must be present at the top level.
	var wire map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &wire); err != nil {
		t.Fatalf("unmarshal wire: %v", err)
	}
	if _, ok := wire["success"]; ok {
		t.Error("legacy success flag must not be emitted")
	}
}

func TestErrorValidationMapsTo400(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Error(c, fmt.Errorf("name: %w", sharederrors.ErrValidation))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	errBody := decodeErr(t, w.Body.String())
	if errBody.Code != "VALIDATION_FAILED" {
		t.Errorf("code = %q, want VALIDATION_FAILED", errBody.Code)
	}
	if errBody.Message == "" {
		t.Error("message should be non-empty")
	}
}

func TestErrorNotFoundMapsTo404(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Error(c, fmt.Errorf("user 5: %w", sharederrors.ErrNotFound))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestErrorUnknownMapsTo500(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Error(c, fmt.Errorf("boom"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	errBody := decodeErr(t, w.Body.String())
	if errBody.Message != "internal server error" {
		t.Errorf("message = %q, want generic internal message", errBody.Message)
	}
	if errBody.Code != "INTERNAL_ERROR" {
		t.Errorf("code = %q, want INTERNAL_ERROR", errBody.Code)
	}
}

func TestErrorStableCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"validation", fmt.Errorf("bad input: %w", sharederrors.ErrValidation), "VALIDATION_FAILED"},
		{"unauthorized", sharederrors.ErrUnauthorized, "UNAUTHORIZED"},
		{"forbidden", sharederrors.ErrForbidden, "FORBIDDEN"},
		{"not found", sharederrors.ErrNotFound, "NOT_FOUND"},
		{"conflict", sharederrors.ErrConflict, "CONFLICT"},
		{"insufficient stock", sharederrors.ErrInsufficientStock, "INSUFFICIENT_STOCK"},
		{"duplicate request", sharederrors.ErrDuplicateRequest, "DUPLICATE_REQUEST"},
		{"rate limited", sharederrors.ErrRateLimited, "RATE_LIMITED"},
		{"unknown", fmt.Errorf("boom"), "INTERNAL_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			Error(c, tc.err)
			errBody := decodeErr(t, w.Body.String())
			if errBody.Code != tc.want {
				t.Errorf("code = %q, want %q", errBody.Code, tc.want)
			}
		})
	}
}

func TestErrorWrappersKeepConflictIdentity(t *testing.T) {
	if !sharederrors.Is(sharederrors.ErrInsufficientStock, sharederrors.ErrConflict) {
		t.Error("ErrInsufficientStock must wrap ErrConflict")
	}
	if !sharederrors.Is(sharederrors.ErrDuplicateRequest, sharederrors.ErrConflict) {
		t.Error("ErrDuplicateRequest must wrap ErrConflict")
	}
}

func TestPaginatedEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Paginated(c, []int{1, 2}, &Pagination{Page: 1, PerPage: 20, Total: 2, TotalPages: 1})

	var body Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Meta == nil || body.Meta.Page != 1 || body.Meta.TotalPages != 1 {
		t.Errorf("meta mismatch: %+v", body.Meta)
	}
	// Wire format must use the PRD §45 `meta` key.
	var wire struct {
		Meta *Pagination `json:"meta"`
		Data []int       `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wire); err != nil {
		t.Fatalf("unmarshal wire: %v", err)
	}
	if wire.Meta == nil || len(wire.Data) != 2 {
		t.Errorf("wire envelope mismatch: %+v", wire)
	}
}
