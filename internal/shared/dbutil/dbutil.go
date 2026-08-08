// Package dbutil provides dependency-free PostgreSQL helpers shared by
// repositories: error classification and pagination normalization.
package dbutil

import "strings"

// IsUniqueViolation reports whether err is a PostgreSQL unique-constraint
// violation (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(),
		"duplicate key value violates unique constraint")
}

// IsForeignKeyViolation reports whether err is a PostgreSQL FK constraint
// violation (SQLSTATE 23503).
func IsForeignKeyViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(),
		"violates foreign key constraint")
}

// NormalizePage clamps page/per-page to the shared defaults and cap:
// page >= 1, perPage in [1, 100] defaulting to 20.
func NormalizePage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}
