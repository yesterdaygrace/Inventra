// Command server is the entrypoint for the Inventra API. It wires
// configuration, logging, and the database into the Gin router.
package main

import (
	"log"
	"net/http"

	"go.uber.org/zap"

	"inventory/internal/auth"
	"inventory/internal/category"
	"inventory/internal/product"
	"inventory/internal/shared/config"
	"inventory/internal/shared/database"
	"inventory/internal/shared/logger"
	"inventory/internal/shared/router"
	"inventory/internal/shared/validator"
	"inventory/internal/user"
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

	if err := database.AutoMigrate(db, database.Models()...); err != nil {
		zlog.Fatal("auto migrate failed", zap.Error(err))
	}

	r := router.New(cfg)

	// Auth module wiring
	authRepo := auth.NewGORMRepository(db)
	tm := auth.NewTokenManager(auth.TokenManagerConfig{
		Secret:     cfg.JWTSecret,
		AccessTTL:  cfg.JWTAccessTTL,
		RefreshTTL: cfg.JWTRefreshTTL,
	})
	authSvc := auth.NewService(authRepo, tm, cfg.BCryptCost)
	authH := auth.NewHandler(authSvc, validator.New())
	auth.RegisterRoutes(r.Group("/api/v1"), authH, tm)

	// User admin module wiring (reuses the auth token parser for RBAC)
	userSvc := user.NewService(user.NewGORMRepository(db))
	userH := user.NewHandler(userSvc, validator.New())
	user.RegisterRoutes(r.Group("/api/v1"), userH, auth.NewTokenParser(tm))

	categorySvc := category.NewService(category.NewGORMRepository(db))
	categoryH := category.NewHandler(categorySvc, validator.New())
	category.RegisterRoutes(r.Group("/api/v1"), categoryH, auth.NewTokenParser(tm))

	productSvc := product.NewService(product.NewGORMRepository(db))
	productH := product.NewHandler(productSvc, validator.New())
	product.RegisterRoutes(r.Group("/api/v1"), productH, auth.NewTokenParser(tm))

	addr := ":" + cfg.Port
	zlog.Info("inventory api listening", zap.String("addr", addr))
	if err := http.ListenAndServe(addr, r); err != nil {
		zlog.Fatal("http server stopped", zap.Error(err))
	}
}
