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
	for _, table := range []string{"workspaces", "sessions", "session_runs", "shortcuts", "goose_agent_db_version"} {
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

func TestMigrateUpgradesCompleteRuntimeSchema(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	if err := Migrate(context.Background(), db, "sqlite"); err != nil {
		t.Fatalf("initial Migrate() error = %v", err)
	}
	if _, err := db.Exec(`DROP TABLE shortcuts`); err != nil {
		t.Fatalf("drop shortcuts for upgrade simulation: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM goose_agent_db_version WHERE version_id IN (202607100001, 202607120001)`); err != nil {
		t.Fatalf("reset shortcut migration versions: %v", err)
	}

	if err := Migrate(context.Background(), db, "sqlite"); err != nil {
		t.Fatalf("upgrade Migrate() error = %v", err)
	}
	if !testTableExists(t, db, "shortcuts") {
		t.Fatal("Migrate() did not add shortcuts to a complete pre-existing runtime schema")
	}
	columns, err := db.Query(`PRAGMA table_info(shortcuts)`)
	if err != nil {
		t.Fatalf("inspect shortcuts columns: %v", err)
	}
	defer func() { _ = columns.Close() }()
	for columns.Next() {
		var position int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := columns.Scan(&position, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan shortcuts column: %v", err)
		}
		if name == "sort_order" {
			return
		}
	}
	if err := columns.Err(); err != nil {
		t.Fatalf("iterate shortcuts columns: %v", err)
	}
	t.Fatal("Migrate() did not add persisted shortcut ordering")
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
