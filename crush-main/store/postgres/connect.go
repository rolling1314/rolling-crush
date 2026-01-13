package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/pkg/config"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

func Connect(ctx context.Context, dataDir string) (*sql.DB, error) {
	// Get configuration
	cfg := config.GetGlobalAppConfig()
	dbCfg := cfg.Database

	// Build PostgreSQL connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dbCfg.Host, dbCfg.Port, dbCfg.User, dbCfg.Password, dbCfg.Database, dbCfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool from config
	db.SetMaxOpenConns(dbCfg.MaxOpenConns)
	db.SetMaxIdleConns(dbCfg.MaxIdleConns)
	db.SetConnMaxLifetime(0)

	slog.Info("Database connection configured",
		"host", dbCfg.Host,
		"port", dbCfg.Port,
		"database", dbCfg.Database,
		"max_open_conns", dbCfg.MaxOpenConns,
		"max_idle_conns", dbCfg.MaxIdleConns,
	)

	// Verify connection
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	goose.SetBaseFS(FS)

	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("Failed to set dialect", "error", err)
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		slog.Error("Failed to apply migrations", "error", err)
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return db, nil
}
