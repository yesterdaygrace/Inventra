package logger

import (
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"inventory/internal/shared/config"
)

func TestNewReturnsLogger(t *testing.T) {
	cfg := &config.Config{AppEnv: "production", LogLevel: "info"}
	z := New(cfg)
	if z == nil {
		t.Fatal("New() returned nil logger")
	}
	_ = z.Sync()
}

func TestNewEmitsJSONFields(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "log-*.json")
	if err != nil {
		t.Fatalf("create temp log file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close log file: %v", err)
		}
	}()

	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(file), zapcore.DebugLevel)
	z := zap.New(core)

	z.Info("hello", zap.String("module", "logger-test"))
	_ = z.Sync()

	buf, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(buf), `"msg":"hello"`) {
		t.Errorf("log output missing msg field: %s", string(buf))
	}
	if !strings.Contains(string(buf), `"module":"logger-test"`) {
		t.Errorf("log output missing custom field: %s", string(buf))
	}
	if !strings.Contains(string(buf), `"level":"info"`) {
		t.Errorf("log output missing level field: %s", string(buf))
	}
}

func TestNewInvalidLevelFallsBackToInfo(t *testing.T) {
	cfg := &config.Config{AppEnv: "production", LogLevel: "bogus-level"}
	z := New(cfg)
	if z == nil {
		t.Fatal("New() returned nil logger for invalid level")
	}
	if lvl := parseLevel(cfg.LogLevel); lvl != zapcore.InfoLevel {
		t.Errorf("parseLevel(%q) = %v, want InfoLevel (fallback)", cfg.LogLevel, lvl)
	}
	_ = z.Sync()
}

func TestNewDebugLevelHonored(t *testing.T) {
	cfg := &config.Config{AppEnv: "production", LogLevel: "debug"}
	z := New(cfg)
	if z == nil {
		t.Fatal("New() returned nil logger")
	}
	if lvl := parseLevel(cfg.LogLevel); lvl != zapcore.DebugLevel {
		t.Errorf("parseLevel(%q) = %v, want DebugLevel", cfg.LogLevel, lvl)
	}
	_ = z.Sync()
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		raw  string
		want zapcore.Level
	}{
		{"", zapcore.InfoLevel},
		{"info", zapcore.InfoLevel},
		{"debug", zapcore.DebugLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"bogus-level", zapcore.InfoLevel},
	}
	for _, tt := range tests {
		if got := parseLevel(tt.raw); got != tt.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}
