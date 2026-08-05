// Package config loads application configuration from environment
// variables and an optional .env file using Viper.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultPort           = "8080"
	defaultBCryptCost     = 12
	defaultLowStock       = 10
	defaultAccessTTL      = 15 * time.Minute
	defaultRefreshTTL     = 7 * 24 * time.Hour
	defaultLogLevel       = "info"
	defaultAppEnv         = "development"
	defaultDBSslMode      = "disable"
	defaultEnvFile        = ".env"
	envFileOverrideVar    = "ENV_FILE"
	accessTTLDefaultStr   = "15m"
	refreshTTLDefaultStr  = "168h"
	accessTTLOverrideKey  = "JWT_ACCESS_TTL"
	refreshTTLOverrideKey = "JWT_REFRESH_TTL"
	requiredPrefix        = "missing required configuration: "
)

// Config holds all runtime configuration for the application.
type Config struct {
	AppEnv            string
	Port              string
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	DBSslMode         string
	JWTSecret         string
	JWTAccessTTL      time.Duration
	JWTRefreshTTL     time.Duration
	BCryptCost        int
	LowStockThreshold int
	CORSOrigins       []string
	LogLevel          string
}

// MissingRequiredError is returned when a required configuration value
// is absent. Callers can use errors.Is against ErrMissingRequired.
type MissingRequiredError struct {
	Key string
}

func (e *MissingRequiredError) Error() string {
	return requiredPrefix + e.Key
}

// ErrMissingRequired matches any MissingRequiredError.
var ErrMissingRequired = &MissingRequiredError{}

// Is implements the errors.Is contract for MissingRequiredError.
func (e *MissingRequiredError) Is(target error) bool {
	_, ok := target.(*MissingRequiredError)
	return ok
}

// Load reads configuration from environment variables and an optional
// .env file (path configurable via ENV_FILE), applies defaults, and
// returns a fully-populated Config. A MissingRequiredError is returned
// when any required variable (DB_USER, DB_PASSWORD, DB_NAME, JWT_SECRET)
// is empty.
func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	envFile := v.GetString(envFileOverrideVar)
	if envFile == "" {
		envFile = defaultEnvFile
	}
	if _, err := os.Stat(envFile); err == nil {
		v.SetConfigFile(envFile)
		v.SetConfigType("env")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read env file %s: %w", envFile, err)
		}
	}

	cfg := &Config{
		AppEnv:            v.GetString("APP_ENV"),
		Port:              v.GetString("PORT"),
		DBHost:            v.GetString("DB_HOST"),
		DBPort:            v.GetString("DB_PORT"),
		DBUser:            v.GetString("DB_USER"),
		DBPassword:        v.GetString("DB_PASSWORD"),
		DBName:            v.GetString("DB_NAME"),
		DBSslMode:         v.GetString("DB_SSLMODE"),
		JWTSecret:         v.GetString("JWT_SECRET"),
		JWTAccessTTL:      v.GetDuration(accessTTLOverrideKey),
		JWTRefreshTTL:     v.GetDuration(refreshTTLOverrideKey),
		BCryptCost:        v.GetInt("BCRYPT_COST"),
		LowStockThreshold: v.GetInt("LOW_STOCK_THRESHOLD"),
		CORSOrigins:       splitOrigins(v.GetString("CORS_ORIGINS")),
		LogLevel:          v.GetString("LOG_LEVEL"),
	}

	// Duration values from viper need explicit parsing when set as strings.
	cfg.JWTAccessTTL = parseDuration(v.GetString(accessTTLOverrideKey), defaultAccessTTL)
	cfg.JWTRefreshTTL = parseDuration(v.GetString(refreshTTLOverrideKey), defaultRefreshTTL)

	if cfg.BCryptCost <= 0 {
		cfg.BCryptCost = defaultBCryptCost
	}
	if cfg.LowStockThreshold <= 0 {
		cfg.LowStockThreshold = defaultLowStock
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", defaultAppEnv)
	v.SetDefault("PORT", defaultPort)
	v.SetDefault("DB_HOST", "localhost")
	v.SetDefault("DB_PORT", "5432")
	v.SetDefault("DB_SSLMODE", defaultDBSslMode)
	v.SetDefault(accessTTLOverrideKey, accessTTLDefaultStr)
	v.SetDefault(refreshTTLOverrideKey, refreshTTLDefaultStr)
	v.SetDefault("BCRYPT_COST", defaultBCryptCost)
	v.SetDefault("LOW_STOCK_THRESHOLD", defaultLowStock)
	v.SetDefault("LOG_LEVEL", defaultLogLevel)
	v.SetDefault("CORS_ORIGINS", "")
	v.SetDefault(envFileOverrideVar, defaultEnvFile)
}

func (c *Config) validate() error {
	required := []struct {
		key   string
		value string
	}{
		{"DB_USER", c.DBUser},
		{"DB_PASSWORD", c.DBPassword},
		{"DB_NAME", c.DBName},
		{"JWT_SECRET", c.JWTSecret},
	}
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			return &MissingRequiredError{Key: r.key}
		}
	}
	return nil
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		// fall back to plain seconds for bare numbers
		if n, nerr := strconv.Atoi(raw); nerr == nil {
			return time.Duration(n) * time.Second
		}
		return fallback
	}
	return d
}

func splitOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isMissingRequired reports whether err is a MissingRequiredError.
func isMissingRequired(err error) bool {
	var mre *MissingRequiredError
	return errors.As(err, &mre)
}
