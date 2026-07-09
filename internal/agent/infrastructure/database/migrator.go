package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"

	migrations "gitee.com/leoninew/TermBridge-go/migrations/agent"

	"github.com/pressly/goose/v3"

	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
)

func Migrate(ctx context.Context, db *sql.DB, driver string) error {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if err := validateRuntimeSchemaState(ctx, db, driver); err != nil {
		return err
	}
	migrationFS, err := fs.Sub(migrations.FS, driver)
	if err != nil {
		return apperrors.Config("select agent database migrations", err)
	}
	goose.SetBaseFS(migrationFS)
	defer goose.SetBaseFS(nil)
	goose.SetTableName("goose_agent_db_version")
	defer goose.SetTableName("goose_db_version")
	if err := goose.SetDialect(gooseDialect(driver)); err != nil {
		return apperrors.Config("set agent migration dialect", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return apperrors.Config("run agent database migrations", err)
	}
	return nil
}

func Version(ctx context.Context, db *sql.DB, driver string) (int64, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	migrationFS, err := fs.Sub(migrations.FS, driver)
	if err != nil {
		return 0, err
	}
	goose.SetBaseFS(migrationFS)
	defer goose.SetBaseFS(nil)
	goose.SetTableName("goose_agent_db_version")
	defer goose.SetTableName("goose_db_version")
	if err := goose.SetDialect(gooseDialect(driver)); err != nil {
		return 0, err
	}
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return 0, fmt.Errorf("get agent migration version: %w", err)
	}
	return version, nil
}

func validateRuntimeSchemaState(ctx context.Context, db *sql.DB, driver string) error {
	workspaces, err := tableExists(ctx, db, driver, "workspaces")
	if err != nil {
		return apperrors.Config("inspect agent schema state", err)
	}
	sessions, err := tableExists(ctx, db, driver, "sessions")
	if err != nil {
		return apperrors.Config("inspect agent schema state", err)
	}
	sessionRuns, err := tableExists(ctx, db, driver, "session_runs")
	if err != nil {
		return apperrors.Config("inspect agent schema state", err)
	}
	if (workspaces || sessions || sessionRuns) && (!workspaces || !sessions || !sessionRuns) {
		return apperrors.Config("invalid agent schema state", fmt.Errorf("runtime tables are partially present"))
	}
	return nil
}

func gooseDialect(driver string) string {
	if driver == "sqlite" {
		return "sqlite3"
	}
	return driver
}

func tableExists(ctx context.Context, db *sql.DB, driver string, table string) (bool, error) {
	switch driver {
	case "sqlite":
		var name string
		err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err == nil {
			return true, nil
		}
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	case "mysql":
		var name string
		err := db.QueryRowContext(ctx, `SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table).Scan(&name)
		if err == nil {
			return true, nil
		}
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	default:
		return false, fmt.Errorf("unsupported driver %q", driver)
	}
}
