// Command migrate applies the versioned SQL migrations in migrations/
// against the configured PostgreSQL database. It is the production schema
// owner — GORM AutoMigrate stays enabled only for local dev (DB_AUTOMIGRATE).
//
// Usage:
//
//	migrate up          apply all pending migrations
//	migrate down        roll back all migrations (to an empty schema)
//	migrate status      print the current schema version
//	migrate version     print the current schema version (alias of status)
//
// The database connection is read from the same environment variables as
// the server (DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE).
package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"inventory/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: migrate <up|down|status|version>")
	}

	m, err := newMigrator()
	if err != nil {
		return err
	}
	defer func() {
		_, _ = m.Close() // best-effort; migration state already persisted
	}()

	switch args[0] {
	case "up":
		return runUp(m)
	case "down":
		return runDown(m)
	case "status", "version":
		return runStatus(m)
	default:
		return fmt.Errorf("unknown subcommand %q (want up|down|status|version)", args[0])
	}
}

func newMigrator() (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("load embedded migrations: %w", err)
	}

	dbURL := postgresURL()
	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return nil, fmt.Errorf("init migrator: %w", err)
	}
	return m, nil
}

func runUp(m *migrate.Migrate) error {
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("no change: schema is already up to date")
			return nil
		}
		return fmt.Errorf("apply migrations: %w", err)
	}
	fmt.Println("migrations applied")
	return nil
}

func runDown(m *migrate.Migrate) error {
	if err := m.Down(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("no change: schema is already at version < 1")
			return nil
		}
		return fmt.Errorf("roll back migrations: %w", err)
	}
	fmt.Println("migrations rolled back")
	return nil
}

func runStatus(m *migrate.Migrate) error {
	version, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if dirty {
		return fmt.Errorf("schema is DIRTY at version %d — manual intervention required", version)
	}
	fmt.Printf("version: %d\n", version)
	return nil
}

// postgresURL builds a postgres:// DSN from the server's env vars.
func postgresURL() string {
	get := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	u := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			get("DB_USER", "postgres"),
			get("DB_PASSWORD", "postgres"),
		),
		Host: get("DB_HOST", "localhost") + ":" + get("DB_PORT", "5432"),
		Path: get("DB_NAME", "inventory"),
	}
	q := u.Query()
	q.Set("sslmode", get("DB_SSLMODE", "disable"))
	u.RawQuery = q.Encode()
	return u.String()
}