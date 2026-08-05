package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// ensure no .env file interferes with the default assertions
	t.Setenv("ENV_FILE", "")
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with all required env set should succeed, got error: %v", err)
	}

	if cfg.BCryptCost != 12 {
		t.Errorf("BCryptCost default = %d, want 12", cfg.BCryptCost)
	}
	if cfg.LowStockThreshold != 10 {
		t.Errorf("LowStockThreshold default = %d, want 10", cfg.LowStockThreshold)
	}
	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Errorf("JWTAccessTTL default = %v, want 15m", cfg.JWTAccessTTL)
	}
	if cfg.JWTRefreshTTL != 7*24*time.Hour {
		t.Errorf("JWTRefreshTTL default = %v, want 168h", cfg.JWTRefreshTTL)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port default = %q, want 8080", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel default = %q, want info", cfg.LogLevel)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("ENV_FILE", "")
	setRequiredEnv(t)
	t.Setenv("PORT", "9090")
	t.Setenv("BCRYPT_COST", "14")
	t.Setenv("LOW_STOCK_THRESHOLD", "5")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("APP_ENV", "test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090 (env override)", cfg.Port)
	}
	if cfg.BCryptCost != 14 {
		t.Errorf("BCryptCost = %d, want 14 (env override)", cfg.BCryptCost)
	}
	if cfg.LowStockThreshold != 5 {
		t.Errorf("LowStockThreshold = %d, want 5 (env override)", cfg.LowStockThreshold)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug (env override)", cfg.LogLevel)
	}
	if cfg.AppEnv != "test" {
		t.Errorf("AppEnv = %q, want test (env override)", cfg.AppEnv)
	}
}

func TestLoadMissingRequiredReturnsError(t *testing.T) {
	// remove one required variable at a time and expect a typed error
	required := []string{"DB_USER", "DB_PASSWORD", "DB_NAME", "JWT_SECRET"}
	for _, key := range required {
		t.Run(key, func(t *testing.T) {
			// start from a full valid env, then unset only `key`
			setRequiredEnv(t)
			// use a temp empty env file so a stray .env can't mask the failure
			t.Setenv("ENV_FILE", "")
			t.Setenv(key, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() should fail when %s is missing", key)
			}
			if !isMissingRequired(err) {
				t.Errorf("error for missing %s should be typed missing-required, got %T %v", key, err, err)
			}
		})
	}
}

func TestLoadFromEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.test")
	content := "PORT=7777\nLOW_STOCK_THRESHOLD=3\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	t.Setenv("ENV_FILE", envPath)
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Port != "7777" {
		t.Errorf("Port from env file = %q, want 7777", cfg.Port)
	}
	if cfg.LowStockThreshold != 3 {
		t.Errorf("LowStockThreshold from env file = %d, want 3", cfg.LowStockThreshold)
	}
}

func TestInvalidIntFallsBackToDefault(t *testing.T) {
	t.Setenv("ENV_FILE", "")
	t.Setenv("BCRYPT_COST", "not-a-number")
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.BCryptCost != 12 {
		t.Errorf("BCryptCost with invalid env = %d, want default 12", cfg.BCryptCost)
	}
}

// setRequiredEnv sets every required variable to a valid value.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_NAME", "inventory")
	t.Setenv("JWT_SECRET", "test-secret")
}
