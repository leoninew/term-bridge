package devicerepo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestRepositoryUpsertsListsAndDeletesUserDeviceBindings(t *testing.T) {
	repo, db := newTestRepository(t)
	ctx := context.Background()
	insertTestUser(t, db, "user-1")
	insertTestUser(t, db, "user-2")

	if err := repo.UpsertDeviceBinding(ctx, "user-1", Device{ID: "dev-1", Name: "local", PublicKey: "key-1"}); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-1) error = %v", err)
	}
	if err := repo.UpsertDeviceBinding(ctx, "user-2", Device{ID: "dev-2", Name: "remote", PublicKey: "key-2"}); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-2) error = %v", err)
	}
	devices, err := repo.ListDevicesForUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListDevicesForUser() error = %v", err)
	}
	if len(devices) != 1 || devices[0].ID != "dev-1" || devices[0].PublicKey != "key-1" {
		t.Fatalf("devices for user-1 = %#v", devices)
	}
	publicKey, err := repo.PublicKey(ctx, "dev-1")
	if err != nil {
		t.Fatalf("PublicKey(dev-1) error = %v", err)
	}
	if publicKey != "key-1" {
		t.Fatalf("PublicKey(dev-1) = %q, want key-1", publicKey)
	}
	owns, err := repo.UserOwnsDevice(ctx, "user-1", "dev-2")
	if err != nil {
		t.Fatalf("UserOwnsDevice() error = %v", err)
	}
	if owns {
		t.Fatal("user-1 unexpectedly owns dev-2")
	}

	if err := repo.DeleteUserDevice(ctx, "user-1", "dev-1"); err != nil {
		t.Fatalf("DeleteUserDevice() error = %v", err)
	}
	devices, err = repo.ListDevicesForUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListDevicesForUser(after delete) error = %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("devices for user-1 after delete = %#v", devices)
	}
	if _, err := repo.PublicKey(ctx, "dev-1"); err == nil {
		t.Fatal("PublicKey(dev-1) error = nil, want deleted key error")
	}
}

func TestRepositoryUpsertsLocalDeviceWithoutUserBinding(t *testing.T) {
	repo, db := newTestRepository(t)
	ctx := context.Background()

	stored, err := repo.UpsertLocalDevice(ctx, Device{ID: "dev-local", Name: "local", PublicKey: "key-local"})
	if err != nil {
		t.Fatalf("UpsertLocalDevice() error = %v", err)
	}
	if stored.ID != "dev-local" || stored.Name != "local" || stored.PublicKey != "key-local" {
		t.Fatalf("UpsertLocalDevice() = %#v", stored)
	}
	var bindingCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_devices WHERE device_id=?`, "dev-local").Scan(&bindingCount); err != nil {
		t.Fatalf("query user_devices count error = %v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("user_devices count for local device = %d, want 0", bindingCount)
	}

	updated, err := repo.UpsertLocalDevice(ctx, Device{ID: "dev-local", Name: "renamed"})
	if err != nil {
		t.Fatalf("UpsertLocalDevice(update) error = %v", err)
	}
	if updated.Name != "renamed" || updated.PublicKey != "key-local" {
		t.Fatalf("UpsertLocalDevice(update) = %#v, want renamed with preserved key", updated)
	}
}

func TestRepositoryBindingCodeIsSingleUseAndExpires(t *testing.T) {
	repo, db := newTestRepository(t)
	ctx := context.Background()
	insertTestUser(t, db, "user-1")

	code, err := repo.CreateBindingCode(ctx, "user-1", time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatalf("CreateBindingCode() error = %v", err)
	}
	userId, ok, err := repo.UseBindingCode(ctx, code)
	if err != nil {
		t.Fatalf("UseBindingCode() error = %v", err)
	}
	if !ok || userId != "user-1" {
		t.Fatalf("UseBindingCode() = %q, %v; want user-1, true", userId, ok)
	}
	if userId, ok, err := repo.UseBindingCode(ctx, code); err != nil || ok || userId != "" {
		t.Fatalf("UseBindingCode(reuse) = %q, %v, %v; want empty, false, nil", userId, ok, err)
	}

	expiredCode, err := repo.CreateBindingCode(ctx, "user-1", time.Now().UTC().Add(-time.Minute))
	if err != nil {
		t.Fatalf("CreateBindingCode(expired) error = %v", err)
	}
	if userId, ok, err := repo.UseBindingCode(ctx, expiredCode); err != nil || ok || userId != "" {
		t.Fatalf("UseBindingCode(expired) = %q, %v, %v; want empty, false, nil", userId, ok, err)
	}
}

func newTestRepository(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Open sqlite error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email_normalized TEXT NOT NULL UNIQUE, display_name TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, email_verified_at TEXT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, last_login_at TEXT NULL)`,
		`CREATE TABLE devices (id TEXT PRIMARY KEY, name TEXT NOT NULL, public_key TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE user_devices (user_id TEXT NOT NULL, device_id TEXT NOT NULL, role TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (user_id, device_id), FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE, FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE)`,
		`CREATE TABLE device_binding_codes (code_hash TEXT PRIMARY KEY, user_id TEXT NOT NULL, used_at TEXT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL, FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec schema statement error = %v\n%s", err, statement)
		}
	}
	return New(db, "sqlite"), db
}

func insertTestUser(t *testing.T, db *sql.DB, userId string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users (id,email_normalized,display_name,status,email_verified_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, userId, userId+"@example.test", userId, "enabled", now, now, now); err != nil {
		t.Fatalf("insert test user %s error = %v", userId, err)
	}
}
