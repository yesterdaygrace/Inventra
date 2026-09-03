// Package migrations embeds the SQL migration files so the migrate CLI
// can run them from the compiled binary without a runtime filesystem
// dependency. golang-migrate's iofs source reads these files.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
