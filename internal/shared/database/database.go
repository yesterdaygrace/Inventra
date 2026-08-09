// Package database establishes the GORM connection to PostgreSQL using
// the pgx driver, with a bounded retry/backoff loop at boot.
package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"inventory/internal/shared/config"
)

const (
	maxRetries        = 5
	retryInterval     = 2 * time.Second
	maxOpenConns      = 25
	maxIdleConns      = 10
	connMaxLifetime   = 5 * time.Minute
	connMaxIdleTime   = 3 * time.Minute
)

// Connect opens a GORM connection to PostgreSQL. It retries with a
// 2s backoff up to maxRetries so the app can outlive a Postgres
// container that is still becoming healthy. The connection pool is
// bounded and the GORM log level is wired from LOG_LEVEL.
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
			Logger: logger.Default.LogMode(gormLogLevel(cfg.LogLevel)),
		})
		if err == nil {
			var sqlDB *sql.DB
			if sqlDB, err = db.DB(); err == nil {
				sqlDB.SetMaxOpenConns(maxOpenConns)
				sqlDB.SetMaxIdleConns(maxIdleConns)
				sqlDB.SetConnMaxLifetime(connMaxLifetime)
				sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
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

// gormLogLevel maps the configured LOG_LEVEL string onto a GORM logger level.
func gormLogLevel(level string) logger.LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return logger.Info
	case "info":
		return logger.Info
	case "error":
		return logger.Error
	case "warn", "warning":
		return logger.Warn
	default:
		return logger.Warn
	}
}

// AutoMigrate runs GORM AutoMigrate for the provided models.
func AutoMigrate(db *gorm.DB, models ...any) error {
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}
