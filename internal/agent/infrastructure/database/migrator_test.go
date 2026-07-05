package database

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateCreatesAgentRuntimeSchemaOnly(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	if err := Migrate(context.Background(), db, "sqlite"); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	for _, table := range []string{"workspaces", "sessions", "session_runs", "goose_agent_db_version"} {
		if !testTableExists(t, db, table) {
			t.Fatalf("table %s does not exist", table)
		}
	}
	for _, table := range []string{"users", "user_identities", "auth_codes", "user_devices", "goose_cloud_db_version"} {
		if testTableExists(t, db, table) {
			t.Fatalf("cloud table %s exists in agent schema", table)
		}
	}
}

func TestMigrateRejectsPartialAgentSchemaState(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE workspaces (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create partial schema table: %v", err)
	}

	err := Migrate(context.Background(), db, "sqlite")
	if err == nil {
		t.Fatal("Migrate() error = nil, want partial schema state error")
	}
	if !strings.Contains(err.Error(), "partially present") {
		t.Fatalf("Migrate() error = %v, want partial schema state error", err)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		t.Fatalf("enable foreign keys: %v", err)
	}
	return db
}

func testTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
	if err == nil {
		return true
	}
	if err == sql.ErrNoRows {
		return false
	}
	t.Fatalf("query table %s: %v", table, err)
	return false
}

func testColumnExists(t *testing.T, db *sql.DB, table string, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
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
			t.Fatalf("scan column: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate columns: %v", err)
	}
	return false
}
