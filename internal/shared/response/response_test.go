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
	if !body.Success {
		t.Error("success should be true")
	}
	data, ok := body.Data.(map[string]any)
	if !ok || data["id"] != float64(1) {
		t.Errorf("data mismatch: %#v", body.Data)
	}
	if body.Pagination != nil {
		t.Error("pagination should be nil for OK")
	}
}

func TestErrorValidationMapsTo400(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Error(c, fmt.Errorf("name: %w", sharederrors.ErrValidation))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var body Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Success {
		t.Error("success should be false")
	}
	if body.Message == "" {
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
	var body Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Message != "internal server error" {
		t.Errorf("message = %q, want generic internal message", body.Message)
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
	if body.Pagination == nil || body.Pagination.Page != 1 || body.Pagination.TotalPages != 1 {
		t.Errorf("pagination mismatch: %+v", body.Pagination)
	}
}
