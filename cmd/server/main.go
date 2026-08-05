// Command server is the entrypoint for the Inventra API. It wires
// configuration, logging, and the database into the Gin router.
package main

import (
	"log"
	"net/http"

	"go.uber.org/zap"

	"inventory/internal/shared/config"
	"inventory/internal/shared/database"
	"inventory/internal/shared/logger"
	"inventory/internal/shared/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	zlog := logger.New(cfg)

	db, err := database.Connect(cfg)
	if err != nil {
		zlog.Fatal("database connection failed", zap.Error(err))
	}
	sqlDB, err := db.DB()
	if err != nil {
		zlog.Fatal("sql db handle failed", zap.Error(err))
	}
	defer sqlDB.Close()

	r := router.New(cfg)

	addr := ":" + cfg.Port
	zlog.Info("inventory api listening", zap.String("addr", addr))
	if err := http.ListenAndServe(addr, r); err != nil {
		zlog.Fatal("http server stopped", zap.Error(err))
	}
}
