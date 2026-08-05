// Package database establishes the GORM connection to PostgreSQL using
// the pgx driver, with a bounded retry/backoff loop at boot.
package database

import (
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"inventory/internal/shared/config"
)

const (
	maxRetries    = 5
	retryInterval = 2 * time.Second
)

// Connect opens a GORM connection to PostgreSQL. It retries with a
// 2s backoff up to maxRetries so the app can outlive a Postgres
// container that is still becoming healthy.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSslMode,
	)

	var (
		db  *gorm.DB
		err error
	)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err == nil {
			var sqlDB *sql.DB
			if sqlDB, err = db.DB(); err == nil {
				if perr := sqlDB.Ping(); perr == nil {
					return db, nil
				} else {
					err = perr
				}
			}
		}
		if attempt < maxRetries {
			time.Sleep(retryInterval)
		}
	}
	return nil, fmt.Errorf("connect to database after %d attempts: %w", maxRetries, err)
}

// AutoMigrate runs GORM AutoMigrate for the provided models.
func AutoMigrate(db *gorm.DB, models ...any) error {
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}
