package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"

	"termbridge-go/internal/infrastructure/database/migrations"
	apperrors "termbridge-go/internal/infrastructure/errors"
)

func Migrate(ctx context.Context, db *sql.DB, driver string) error {
	migrationFS, err := fs.Sub(migrations.FS, driver)
	if err != nil {
		return apperrors.Config("select database migrations", err)
	}
	goose.SetBaseFS(migrationFS)
	defer goose.SetBaseFS(nil)
	goose.SetTableName("goose_db_version")
	if err := goose.SetDialect(gooseDialect(driver)); err != nil {
		return apperrors.Config("set migration dialect", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return apperrors.Config("run database migrations", err)
	}
	return nil
}

func gooseDialect(driver string) string {
	if driver == "sqlite" {
		return "sqlite3"
	}
	return driver
}

func Version(ctx context.Context, db *sql.DB, driver string) (int64, error) {
	migrationFS, err := fs.Sub(migrations.FS, driver)
	if err != nil {
		return 0, err
	}
	goose.SetBaseFS(migrationFS)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect(gooseDialect(driver)); err != nil {
		return 0, err
	}
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return 0, fmt.Errorf("get migration version: %w", err)
	}
	return version, nil
}
