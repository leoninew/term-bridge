package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"termbridge-go/internal/process"
	"termbridge-go/internal/session"
	"termbridge-go/internal/workspace"
)

func TestStoreSavesAndListsRecords(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, ID: "01", Key: "workspacekey", Name: "project", Path: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, ID: "02", WorkspaceID: ws.ID, WorkspaceKey: ws.Key, LaunchCwd: ws.Path, Command: session.CommandRecord{Command: "pwsh", Args: []string{"-NoLogo"}}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := store.SaveState(ws.Key, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "done", UpdatedAt: now}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	if err := store.SaveProcess(ws.Key, sess.ID, process.Record{SchemaVersion: 1, PID: 123}); err != nil {
		t.Fatalf("SaveProcess() error = %v", err)
	}
	if err := store.SaveExit(ws.Key, sess.ID, process.ExitRecord{SchemaVersion: 1, ExitCode: 7, Reason: "user_process_exited"}); err != nil {
		t.Fatalf("SaveExit() error = %v", err)
	}

	workspaces, warnings, err := store.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(workspaces) != 1 || workspaces[0].ID != ws.ID {
		t.Fatalf("workspaces = %#v", workspaces)
	}

	sessions, warnings, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(sessions) != 1 || sessions[0].Session.ID != sess.ID {
		t.Fatalf("sessions = %#v", sessions)
	}
	if sessions[0].ExitCode == nil || *sessions[0].ExitCode != 7 {
		t.Fatalf("ExitCode = %#v", sessions[0].ExitCode)
	}
}

func TestStoreSkipsBrokenWorkspaceJSON(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	brokenDir := filepath.Join(store.Root, "broken")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "workspace.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	workspaces, warnings, err := store.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(workspaces) != 0 {
		t.Fatalf("workspaces = %#v", workspaces)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want one", warnings)
	}
}
