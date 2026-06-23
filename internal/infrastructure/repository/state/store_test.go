package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"termbridge-go/internal/domain/process"
	"termbridge-go/internal/domain/session"
	"termbridge-go/internal/domain/workspace"
)

func TestStoreSavesAndListsRecords(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "01", Key: "workspacekey", Name: "project", Path: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "02", WorkspaceId: ws.Id, WorkspaceKey: ws.Key, LaunchCwd: ws.Path, Command: session.CommandRecord{Command: "pwsh", Args: []string{"-NoLogo"}}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := store.SaveState(ws.Key, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "done", UpdatedAt: now}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	if err := store.SaveProcess(ws.Key, sess.Id, process.Record{SchemaVersion: 1, Pid: 123}); err != nil {
		t.Fatalf("SaveProcess() error = %v", err)
	}
	if err := store.SaveExit(ws.Key, sess.Id, process.ExitRecord{SchemaVersion: 1, ExitCode: 7, Reason: "user_process_exited"}); err != nil {
		t.Fatalf("SaveExit() error = %v", err)
	}

	workspaces, warnings, err := store.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(workspaces) != 1 || workspaces[0].Id != ws.Id {
		t.Fatalf("workspaces = %#v", workspaces)
	}

	sessions, warnings, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(sessions) != 1 || sessions[0].Session.Id != sess.Id {
		t.Fatalf("sessions = %#v", sessions)
	}
	if sessions[0].ExitCode == nil || *sessions[0].ExitCode != 7 {
		t.Fatalf("ExitCode = %#v", sessions[0].ExitCode)
	}
}

func TestStoreOverwritesExistingState(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	workspaceKey := "workspacekey"
	sessionId := "session"
	first := session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: now}
	second := session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "user_process_exited", UpdatedAt: now.Add(time.Second)}

	if err := store.SaveState(workspaceKey, sessionId, first); err != nil {
		t.Fatalf("SaveState(first) error = %v", err)
	}
	if err := store.SaveState(workspaceKey, sessionId, second); err != nil {
		t.Fatalf("SaveState(second) error = %v", err)
	}
	stored, err := store.LoadState(workspaceKey, sessionId)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if stored.State != session.StateStopped || stored.Reason != "user_process_exited" {
		t.Fatalf("state = %#v, want stopped user_process_exited", stored)
	}
}

func TestStoreOrdersAndDeletesWorkspaceSessions(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	first := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "first", Key: "firstkey", Name: "first", Path: t.TempDir(), SortOrder: 2, CreatedAt: now, UpdatedAt: now}
	second := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "second", Key: "secondkey", Name: "second", Path: t.TempDir(), SortOrder: 1, CreatedAt: now, UpdatedAt: now}
	for _, ws := range []workspace.Workspace{first, second} {
		if err := store.SaveWorkspace(ws); err != nil {
			t.Fatalf("SaveWorkspace() error = %v", err)
		}
	}
	workspaces, _, err := store.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(workspaces) != 2 || workspaces[0].Id != second.Id || workspaces[1].Id != first.Id {
		t.Fatalf("workspaces = %#v", workspaces)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "session", Name: "Session", WorkspaceId: second.Id, WorkspaceKey: second.Key, LaunchCwd: second.Path, Command: session.CommandRecord{Command: "zsh"}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	storedWorkspace, err := store.LoadWorkspace(second.Key)
	if err != nil {
		t.Fatalf("LoadWorkspace() error = %v", err)
	}
	if len(storedWorkspace.Children) != 1 || storedWorkspace.Children[0].Name != "Session" {
		t.Fatalf("workspace children = %#v", storedWorkspace.Children)
	}
	if err := store.SaveState(second.Key, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	views, _, err := store.ListSessionsByWorkspaceId(second.Id)
	if err != nil {
		t.Fatalf("ListSessionsByWorkspaceId() error = %v", err)
	}
	if len(views) != 1 || views[0].Session.Name != "Session" {
		t.Fatalf("views = %#v", views)
	}
	if err := store.DeleteSession(second.Key, sess.Id); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if _, err := store.LoadSession(second.Key, sess.Id); err == nil {
		t.Fatal("LoadSession() error = nil, want deleted session")
	}
	storedWorkspace, err = store.LoadWorkspace(second.Key)
	if err != nil {
		t.Fatalf("LoadWorkspace() after delete session error = %v", err)
	}
	if len(storedWorkspace.Children) != 0 {
		t.Fatalf("workspace children after delete = %#v", storedWorkspace.Children)
	}
	if err := store.DeleteWorkspace(second.Key); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v", err)
	}
	if _, err := store.LoadWorkspace(second.Key); err == nil {
		t.Fatal("LoadWorkspace() error = nil, want deleted workspace")
	}
}

func TestStoreSkipsBrokenWorkspaceJSON(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	brokenDir := filepath.Join(store.WorkspaceRoot(), "broken")
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
