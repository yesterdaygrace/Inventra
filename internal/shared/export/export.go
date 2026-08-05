// Package export is the single source of truth for CSV generation:
// RFC 4180 quoting, CRLF line endings, UTF-8 BOM, and attachment
// headers for the module export endpoints.
package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
)

// WriteCSV writes headers + rows to w as RFC 4180 CSV with a UTF-8 BOM.
func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("write bom: %w", err)
	}
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	if err := cw.Write(headers); err != nil {
		return fmt.Errorf("write csv headers: %w", err)
	}
	if err := cw.WriteAll(rows); err != nil {
		return fmt.Errorf("write csv rows: %w", err)
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

// SetAttachment sets the text/csv content type and an attachment
// Content-Disposition with a module_YYYYMMDDHHMM.csv filename.
func SetAttachment(c *gin.Context, module string) {
	name := fmt.Sprintf("%s_%s.csv", module, time.Now().Format("200601021504"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
}
