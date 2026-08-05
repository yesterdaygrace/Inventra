package database

import (
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"inventory/internal/shared/config"
)

func testConfig() *config.Config {
	return &config.Config{
		DBHost:     envOr("DB_HOST", "localhost"),
		DBPort:     envOr("DB_PORT", "5433"),
		DBUser:     envOr("DB_USER", "postgres"),
		DBPassword: envOr("DB_PASSWORD", "postgres"),
		DBName:     envOr("DB_NAME", "inventory"),
		DBSslMode:  "disable",
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestConnectWithDefaultConfig(t *testing.T) {
	db, err := Connect(testConfig())
	if err != nil {
		t.Fatalf("Connect() against dockerized postgres should succeed, got: %v", err)
	}
	if db == nil {
		t.Fatal("Connect() returned nil db")
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() failed: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	_ = sqlDB.Close()
}

func TestConnectWrongDatabaseReturnsError(t *testing.T) {
	cfg := testConfig()
	cfg.DBName = "definitely_not_a_real_db_xyz"
	start := time.Now()
	_, err := Connect(cfg)
	if err == nil {
		t.Fatal("Connect() with wrong db name should return an error")
	}
	if time.Since(start) > 30*time.Second {
		t.Error("Connect() took too long; retry/backoff should give up within ~30s")
	}
}

func TestAutoMigrateCreatesTables(t *testing.T) {
	db, err := Connect(testConfig())
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() failed: %v", err)
	}
	defer sqlDB.Close()

	probe := &gorm.Model{}
	if err := db.AutoMigrate(probe); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	if !db.Migrator().HasTable("models") {
		t.Error("AutoMigrate did not create the expected table")
	}
	_ = db.Migrator().DropTable(probe)
}
