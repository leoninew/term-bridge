package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"

	"termbridge-go/internal/infrastructure/config"
	apperrors "termbridge-go/internal/infrastructure/errors"
)

type DB struct {
	*sql.DB
	Driver string
}

func Open(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	switch cfg.Driver {
	case "sqlite":
		return openSQLite(ctx, cfg.SQLite.Path)
	case "mysql":
		return openMySQL(ctx, cfg.MySQL.DSN)
	default:
		return nil, apperrors.Config("open database", fmt.Errorf("unsupported driver %q", cfg.Driver))
	}
}

func openSQLite(ctx context.Context, path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, apperrors.Config("create sqlite parent directory", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, apperrors.Config("open sqlite database", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, apperrors.Config("set sqlite busy_timeout", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, apperrors.Config("set sqlite foreign_keys", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, apperrors.Config("set sqlite journal_mode", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, apperrors.Config("ping sqlite database", err)
	}
	return &DB{DB: db, Driver: "sqlite"}, nil
}

func openMySQL(ctx context.Context, dsn string) (*DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, apperrors.Config("open mysql database", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, apperrors.Config("ping mysql database", err)
	}
	return &DB{DB: db, Driver: "mysql"}, nil
}
