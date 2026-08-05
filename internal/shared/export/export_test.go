package export

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWriteCSVQuoting(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"name", "description", "qty"}
	rows := [][]string{
		{"Widget", "A, " + "B", "10"},
		{"Gadget", "has \"quotes\"", "5"},
		{"Thing", "multi\nline", "0"},
	}

	if err := WriteCSV(&buf, headers, rows); err != nil {
		t.Fatalf("WriteCSV failed: %v", err)
	}

	out := buf.String()
	if !bytes.HasPrefix([]byte(out), []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("missing BOM: %q", out)
	}
	body := bytes.TrimPrefix(buf.Bytes(), []byte{0xEF, 0xBB, 0xBF})
	if !bytes.HasPrefix(body, []byte("name,description,qty\r\n")) {
		t.Fatalf("header row malformed: %q", out)
	}
	if !bytes.Contains([]byte(out), []byte("\"A, B\"")) {
		t.Errorf("expected quoted comma field, got: %q", out)
	}
	if !bytes.Contains([]byte(out), []byte("\"has \"\"quotes\"\"\"")) {
		t.Errorf("expected escaped quotes, got: %q", out)
	}
	if !bytes.Contains([]byte(out), []byte("\"multi\r\nline\"")) {
		t.Errorf("expected quoted CRLF newline, got: %q", out)
	}
	if !bytes.Contains([]byte(out), []byte("\r\n")) {
		t.Error("CRLF line ending missing")
	}
}

func TestWriteCSVIncludesBOM(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, []string{"a"}, [][]string{{"1"}}); err != nil {
		t.Fatalf("WriteCSV failed: %v", err)
	}
	// UTF-8 BOM EF BB BF
	want := []byte{0xEF, 0xBB, 0xBF}
	if !bytes.HasPrefix(buf.Bytes(), want) {
		t.Errorf("missing UTF-8 BOM: first bytes % x", buf.Bytes()[:3])
	}
}

func TestContentDispositionHelper(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SetAttachment(c, "products")
	cd := w.Header().Get("Content-Disposition")
	if cd == "" {
		t.Fatal("Content-Disposition not set")
	}
	if w.Header().Get("Content-Type") != "text/csv" {
		t.Errorf("Content-Type = %q, want text/csv", w.Header().Get("Content-Type"))
	}
}

func TestContentDispositionFilename(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SetAttachment(c, "products")
	cd := w.Header().Get("Content-Disposition")
	// expect attachment; filename="products_YYYYMMDDHHMM.csv"
	want := "attachment; filename=\"products_"
	if len(cd) < len(want) || cd[:len(want)] != want {
		t.Errorf("Content-Disposition = %q, want prefix %q", cd, want)
	}
	if len(cd) != len("attachment; filename=\"products_200601021504.csv\"") {
		t.Errorf("Content-Disposition unexpected length: %q", cd)
	}
}
