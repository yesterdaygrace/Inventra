// Package logger provides structured logging via Zap. It exposes a single
// New(cfg) constructor that returns a production-grade *zap.Logger with a
// JSON encoder, wired to the configured LOG_LEVEL.
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"inventory/internal/shared/config"
)

// New builds a *zap.Logger from config. In development (APP_ENV=development)
// a human-friendly console encoder is used; otherwise JSON. An unknown
// LOG_LEVEL falls back to info instead of panicking.
func New(cfg *config.Config) *zap.Logger {
	level := parseLevel(cfg.LogLevel)

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var core zapcore.Core
	if cfg.AppEnv == "development" {
		consoleCfg := zap.NewDevelopmentEncoderConfig()
		consoleCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		core = zapcore.NewCore(zapcore.NewConsoleEncoder(consoleCfg), zapcore.Lock(os.Stdout), level)
	} else {
		core = zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.Lock(os.Stdout), level)
	}

	return zap.New(core, zap.AddCaller())
}

func parseLevel(raw string) zapcore.Level {
	if raw == "" {
		return zapcore.InfoLevel
	}
	lvl, err := zapcore.ParseLevel(raw)
	if err != nil {
		return zapcore.InfoLevel
	}
	return lvl
}
