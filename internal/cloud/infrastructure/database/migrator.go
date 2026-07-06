package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"
	migrations "termbridge/migrations/agent"

	"github.com/pressly/goose/v3"

	apperrors "termbridge/internal/shared/common/errors"
)

func Migrate(ctx context.Context, db *sql.DB, driver string) error {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if err := validateCloudSchemaState(ctx, db, driver); err != nil {
		return err
	}
	migrationFS, err := fs.Sub(migrations.FS, driver)
	if err != nil {
		return apperrors.Config("select cloud database migrations", err)
	}
	goose.SetBaseFS(migrationFS)
	defer goose.SetBaseFS(nil)
	goose.SetTableName("goose_cloud_db_version")
	defer goose.SetTableName("goose_db_version")
	if err := goose.SetDialect(gooseDialect(driver)); err != nil {
		return apperrors.Config("set cloud migration dialect", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return apperrors.Config("run cloud database migrations", err)
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
	goose.SetTableName("goose_cloud_db_version")
	defer goose.SetTableName("goose_db_version")
	if err := goose.SetDialect(gooseDialect(driver)); err != nil {
		return 0, err
	}
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return 0, fmt.Errorf("get cloud migration version: %w", err)
	}
	return version, nil
}

func validateCloudSchemaState(ctx context.Context, db *sql.DB, driver string) error {
	groups := [][]string{
		{"users", "user_identities", "auth_codes", "email_delivery_logs", "oauth_states"},
		{"devices", "user_devices"},
	}
	for _, group := range groups {
		present := 0
		for _, table := range group {
			exists, err := tableExists(ctx, db, driver, table)
			if err != nil {
				return apperrors.Config("inspect cloud schema state", err)
			}
			if exists {
				present++
			}
		}
		if present > 0 && present != len(group) {
			return apperrors.Config("invalid cloud schema state", fmt.Errorf("tables are partially present: %s", strings.Join(group, ", ")))
		}
	}
	devices, err := tableExists(ctx, db, driver, "devices")
	if err != nil {
		return apperrors.Config("inspect cloud schema state", err)
	}
	if devices {
		publicKey, err := columnExists(ctx, db, driver, "devices", "public_key")
		if err != nil {
			return apperrors.Config("inspect cloud schema state", err)
		}
		if !publicKey {
			return apperrors.Config("invalid cloud schema state", fmt.Errorf("devices table exists without public_key column"))
		}
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

func columnExists(ctx context.Context, db *sql.DB, driver string, table string, column string) (bool, error) {
	switch driver {
	case "sqlite":
		rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
		if err != nil {
			return false, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var cid int
			var name string
			var typ string
			var notNull int
			var defaultValue sql.NullString
			var pk int
			if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
				return false, err
			}
			if name == column {
				return true, nil
			}
		}
		return false, rows.Err()
	case "mysql":
		var name string
		err := db.QueryRowContext(ctx, `SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, table, column).Scan(&name)
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
