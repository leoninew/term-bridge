package database

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateCreatesCloudIdentitySchemaOnly(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := Migrate(context.Background(), db, "sqlite"); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	for _, table := range []string{"users", "user_identities", "auth_codes", "email_delivery_logs", "oauth_states", "devices", "user_devices", "device_binding_codes", "goose_cloud_db_version"} {
		if !testTableExists(t, db, table) {
			t.Fatalf("table %s does not exist", table)
		}
	}
	for _, table := range []string{"workspaces", "sessions", "session_runs", "goose_agent_db_version"} {
		if testTableExists(t, db, table) {
			t.Fatalf("agent table %s exists in cloud schema", table)
		}
	}
	if !testColumnExists(t, db, "devices", "public_key") {
		t.Fatal("devices.public_key does not exist")
	}
}

func TestMigrateRejectsPartialCloudSchemaState(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY)`); err != nil {
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

func TestMigrateRejectsDeviceWithoutPublicKey(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create users table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE user_identities (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create user_identities table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE auth_codes (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create auth_codes table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE email_delivery_logs (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create email_delivery_logs table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE oauth_states (state TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create oauth_states table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE devices (id TEXT PRIMARY KEY, name TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create devices table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE user_devices (user_id TEXT NOT NULL, device_id TEXT NOT NULL, PRIMARY KEY (user_id, device_id))`); err != nil {
		t.Fatalf("create user_devices table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE device_binding_codes (code_hash TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create device_binding_codes table: %v", err)
	}

	err := Migrate(context.Background(), db, "sqlite")
	if err == nil {
		t.Fatal("Migrate() error = nil, want public_key schema state error")
	}
	if !strings.Contains(err.Error(), "public_key") {
		t.Fatalf("Migrate() error = %v, want public_key error", err)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
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
	defer rows.Close()
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
