package state

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/session"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/shortcut"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"

	_ "modernc.org/sqlite"
)

func TestDBStoreSavesListsAndSoftDeletesRuntimeState(t *testing.T) {
	store, db := newTestDBStore(t)
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace-1", Name: "project", Path: filepath.Join(t.TempDir(), "project"), CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "session-1", WorkspaceId: ws.Id, Name: "shell", LaunchCwd: ws.Path, Command: session.CommandRecord{Command: "pwsh -NoLogo", EnvStrategy: "inherit", EnvCount: 3}, History: session.HistoryRecord{Path: "history.log", MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := store.SaveProcess(ws.Id, sess.Id, process.Record{SchemaVersion: 1, Pid: 123, CommandLine: "pwsh -NoLogo", StartedAt: now}); err != nil {
		t.Fatalf("SaveProcess() error = %v", err)
	}
	if err := store.SaveExit(ws.Id, sess.Id, process.ExitRecord{SchemaVersion: 1, ExitCode: 7, Reason: "user_process_exited", StartedAt: now, EndedAt: now.Add(time.Second)}); err != nil {
		t.Fatalf("SaveExit() error = %v", err)
	}
	if err := store.SaveState(ws.Id, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "user_process_exited", UpdatedAt: now.Add(time.Second)}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	workspaces, warnings, err := store.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(workspaces) != 1 || workspaces[0].Id != ws.Id || len(workspaces[0].Children) != 1 {
		t.Fatalf("workspaces = %#v", workspaces)
	}
	views, warnings, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(views) != 1 || views[0].Session.Id != sess.Id || views[0].State.State != session.StateStopped {
		t.Fatalf("views = %#v", views)
	}
	if views[0].ExitCode == nil || *views[0].ExitCode != 7 {
		t.Fatalf("ExitCode = %#v, want 7", views[0].ExitCode)
	}

	var runCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM session_runs WHERE session_id=?`, sess.Id).Scan(&runCount); err != nil {
		t.Fatalf("query run count error = %v", err)
	}
	if runCount != 1 {
		t.Fatalf("session run count = %d, want 1", runCount)
	}

	if err := store.DeleteSession(ws.Id, sess.Id); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if _, err := store.LoadSession(ws.Id, sess.Id); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadSession(deleted) error = %v, want os.ErrNotExist", err)
	}
	var deletedAt sql.NullString
	if err := db.QueryRow(`SELECT deleted_at FROM sessions WHERE id=?`, sess.Id).Scan(&deletedAt); err != nil {
		t.Fatalf("query deleted_at error = %v", err)
	}
	if !deletedAt.Valid || deletedAt.String == "" {
		t.Fatalf("deleted_at = %#v, want soft delete timestamp", deletedAt)
	}
}

func TestDBStoreReusesActiveWorkspacePath(t *testing.T) {
	store, _ := newTestDBStore(t)
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "project")
	first := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace-1", Name: "first", Path: path, CreatedAt: now, UpdatedAt: now}
	second := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace-2", Name: "second", Path: path, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)}
	if err := store.SaveWorkspace(first); err != nil {
		t.Fatalf("SaveWorkspace(first) error = %v", err)
	}
	if err := store.SaveWorkspace(second); err != nil {
		t.Fatalf("SaveWorkspace(second) error = %v", err)
	}
	workspaces, _, err := store.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(workspaces) != 1 || workspaces[0].Id != first.Id || workspaces[0].Name != second.Name {
		t.Fatalf("workspaces = %#v, want first row reused with updated name", workspaces)
	}
}

func TestDBStoreAppendsRunForRerun(t *testing.T) {
	store, db := newTestDBStore(t)
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace-1", Name: "project", Path: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "session-1", WorkspaceId: ws.Id, Name: "shell", LaunchCwd: ws.Path, Command: session.CommandRecord{Command: "bash"}, History: session.HistoryRecord{Path: "history.log"}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := store.SaveState(ws.Id, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "done", UpdatedAt: now.Add(time.Second)}); err != nil {
		t.Fatalf("SaveState(stopped) error = %v", err)
	}
	if err := store.BeginSessionRun(ws.Id, sess.Id, process.TerminalSize{Cols: 100, Rows: 30}); err != nil {
		t.Fatalf("BeginSessionRun() error = %v", err)
	}
	if err := store.SaveProcess(ws.Id, sess.Id, process.Record{SchemaVersion: 1, Pid: 456, StartedAt: now.Add(2 * time.Second)}); err != nil {
		t.Fatalf("SaveProcess(second run) error = %v", err)
	}
	var runCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM session_runs WHERE session_id=?`, sess.Id).Scan(&runCount); err != nil {
		t.Fatalf("query run count error = %v", err)
	}
	if runCount != 2 {
		t.Fatalf("session run count = %d, want 2", runCount)
	}
	processRecord, err := store.LoadProcess(ws.Id, sess.Id)
	if err != nil {
		t.Fatalf("LoadProcess() error = %v", err)
	}
	if processRecord.Pid != 456 {
		t.Fatalf("LoadProcess() = %#v, want current run pid 456", processRecord)
	}
}

func TestDBStoreSavesProcessAndRunningStateTogether(t *testing.T) {
	store, _ := newTestDBStore(t)
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace-1", Name: "project", Path: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "session-1", WorkspaceId: ws.Id, Name: "shell", LaunchCwd: ws.Path, Command: session.CommandRecord{Command: "bash"}, History: session.HistoryRecord{Path: "history.log"}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	stateRecord := session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: now.Add(time.Second)}
	if err := store.SaveProcessState(ws.Id, sess.Id, process.Record{SchemaVersion: 1, Pid: 789, StartedAt: now.Add(time.Second)}, stateRecord); err != nil {
		t.Fatalf("SaveProcessState() error = %v", err)
	}

	gotState, err := store.LoadState(ws.Id, sess.Id)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if gotState.State != session.StateRunning || gotState.Reason != "process_started" {
		t.Fatalf("state = %#v, want running process_started", gotState)
	}
	processRecord, err := store.LoadProcess(ws.Id, sess.Id)
	if err != nil {
		t.Fatalf("LoadProcess() error = %v", err)
	}
	if processRecord.Pid != 789 {
		t.Fatalf("process pid = %d, want 789", processRecord.Pid)
	}
}

func TestDBStoreSavesExitAndFinalStateTogether(t *testing.T) {
	store, _ := newTestDBStore(t)
	now := time.Date(2026, 7, 5, 11, 0, 0, 0, time.UTC)
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace-1", Name: "project", Path: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "session-1", WorkspaceId: ws.Id, Name: "shell", LaunchCwd: ws.Path, Command: session.CommandRecord{Command: "bash"}, History: session.HistoryRecord{Path: "history.log"}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	stateRecord := session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "user_process_exited", UpdatedAt: now.Add(time.Second)}
	if err := store.SaveExitState(ws.Id, sess.Id, process.ExitRecord{SchemaVersion: 1, ExitCode: 1, Reason: "user_process_exited", StartedAt: now, EndedAt: now.Add(time.Second)}, stateRecord); err != nil {
		t.Fatalf("SaveExitState() error = %v", err)
	}

	gotState, err := store.LoadState(ws.Id, sess.Id)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if gotState.State != session.StateFailed || gotState.Reason != "user_process_exited" {
		t.Fatalf("state = %#v, want failed user_process_exited", gotState)
	}
	exitRecord, err := store.LoadExit(ws.Id, sess.Id)
	if err != nil {
		t.Fatalf("LoadExit() error = %v", err)
	}
	if exitRecord.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitRecord.ExitCode)
	}
}

func TestDBStoreOrdersWorkspacesAndSessions(t *testing.T) {
	store, _ := newTestDBStore(t)
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	first := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace-1", Name: "first", Path: filepath.Join(t.TempDir(), "first"), CreatedAt: now, UpdatedAt: now}
	second := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace-2", Name: "second", Path: filepath.Join(t.TempDir(), "second"), CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)}
	for _, ws := range []workspace.Workspace{first, second} {
		if err := store.SaveWorkspace(ws); err != nil {
			t.Fatalf("SaveWorkspace(%s) error = %v", ws.Id, err)
		}
	}
	ordered, err := store.UpdateWorkspaceOrder([]string{second.Id, first.Id}, now)
	if err != nil {
		t.Fatalf("UpdateWorkspaceOrder() error = %v", err)
	}
	if len(ordered) != 2 || ordered[0].Id != second.Id || ordered[1].Id != first.Id {
		t.Fatalf("ordered workspaces = %#v", ordered)
	}
	sessions := []session.Session{
		{SchemaVersion: session.SchemaVersion, Id: "session-1", WorkspaceId: second.Id, Name: "first", LaunchCwd: second.Path, Command: session.CommandRecord{Command: "zsh"}, CreatedAt: now, UpdatedAt: now},
		{SchemaVersion: session.SchemaVersion, Id: "session-2", WorkspaceId: second.Id, Name: "second", LaunchCwd: second.Path, Command: session.CommandRecord{Command: "bash"}, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)},
	}
	for _, sess := range sessions {
		if err := store.SaveSession(sess); err != nil {
			t.Fatalf("SaveSession(%s) error = %v", sess.Id, err)
		}
	}
	views, _, err := store.UpdateSessionOrder(second.Id, []string{"session-2", "session-1"}, now)
	if err != nil {
		t.Fatalf("UpdateSessionOrder() error = %v", err)
	}
	if len(views) != 2 || views[0].Session.Id != "session-2" || views[1].Session.Id != "session-1" {
		t.Fatalf("ordered sessions = %#v", views)
	}
}

func TestDBStorePersistsDeviceScopedShortcuts(t *testing.T) {
	store, db := newTestDBStore(t)
	commandText := `codex --dangerously-bypass-approvals-and-sandbox -c "review changes"`
	description := "  review workflow  "
	created, err := store.CreateShortcut(shortcut.Shortcut{Id: "shortcut-1", Name: "  Review  ", Command: commandText, Description: &description})
	if err != nil {
		t.Fatalf("CreateShortcut() error = %v", err)
	}
	if created.Name != "Review" || created.Command != commandText || created.Description == nil || *created.Description != "review workflow" {
		t.Fatalf("CreateShortcut() = %#v, want normalized metadata with raw command", created)
	}
	listed, err := store.ListShortcuts()
	if err != nil {
		t.Fatalf("ListShortcuts() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Id != created.Id || listed[0].Command != commandText {
		t.Fatalf("ListShortcuts() = %#v, want saved shortcut", listed)
	}
	previousUpdatedAt := created.UpdatedAt
	updated, err := store.UpdateShortcut(created.Id, func(value *shortcut.Shortcut) error {
		value.Command = "cmd"
		value.Description = nil
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateShortcut() error = %v", err)
	}
	if updated.Command != "cmd" || updated.Description != nil || !updated.UpdatedAt.After(previousUpdatedAt) {
		t.Fatalf("UpdateShortcut() = %#v, want updated command, cleared description, and newer timestamp", updated)
	}
	otherDeviceStore := NewDbStore(db, "sqlite", t.TempDir(), "device-2")
	otherDeviceShortcuts, err := otherDeviceStore.ListShortcuts()
	if err != nil {
		t.Fatalf("ListShortcuts(other device) error = %v", err)
	}
	if len(otherDeviceShortcuts) != 0 {
		t.Fatalf("ListShortcuts(other device) = %#v, want no shortcuts", otherDeviceShortcuts)
	}
	if _, err := otherDeviceStore.UpdateShortcut(created.Id, func(*shortcut.Shortcut) error { return nil }); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("UpdateShortcut(other device) error = %v, want os.ErrNotExist", err)
	}
	if err := otherDeviceStore.DeleteShortcut(created.Id); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("DeleteShortcut(other device) error = %v, want os.ErrNotExist", err)
	}
	if err := store.DeleteShortcut(created.Id); err != nil {
		t.Fatalf("DeleteShortcut() error = %v", err)
	}
	listed, err = store.ListShortcuts()
	if err != nil {
		t.Fatalf("ListShortcuts(after delete) error = %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("ListShortcuts(after delete) = %#v, want no selectable shortcut", listed)
	}
}

func TestDBStoreRejectsBlankShortcutRequiredFields(t *testing.T) {
	store, _ := newTestDBStore(t)
	for _, value := range []shortcut.Shortcut{{Name: "", Command: "cmd"}, {Name: "shell", Command: " \t "}} {
		if _, err := store.CreateShortcut(value); err == nil {
			t.Fatalf("CreateShortcut(%#v) error = nil, want required-field error", value)
		}
	}
}

func TestDBStoreUpdatesOnlyStoppedSessions(t *testing.T) {
	store, _ := newTestDBStore(t)
	now := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace-1", Name: "project", Path: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "session-1", WorkspaceId: ws.Id, Name: "shell", LaunchCwd: ws.Path, Command: session.CommandRecord{Command: "cmd"}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if _, err := store.UpdateStoppedSession(ws.Id, sess.Id, func(value *session.Session) error {
		value.Name = "should not apply"
		value.UpdatedAt = now.Add(time.Second)
		return nil
	}); err == nil {
		t.Fatal("UpdateStoppedSession() error = nil while session is running")
	}
	if err := store.SaveState(ws.Id, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, UpdatedAt: now.Add(time.Second)}); err != nil {
		t.Fatalf("SaveState(stopped) error = %v", err)
	}
	updated, err := store.UpdateStoppedSession(ws.Id, sess.Id, func(value *session.Session) error {
		value.Name = "edited shell"
		value.Command.Command = `cmd /c "echo updated"`
		value.UpdatedAt = now.Add(2 * time.Second)
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateStoppedSession() error = %v", err)
	}
	if updated.Name != "edited shell" || updated.Command.Command != `cmd /c "echo updated"` || updated.LaunchCwd != ws.Path {
		t.Fatalf("UpdateStoppedSession() = %#v, want updated name/command and unchanged cwd", updated)
	}
}

func TestDBStoreHistoryPathRemainsFileBackedTerminalOutput(t *testing.T) {
	store, _ := newTestDBStore(t)
	want := filepath.Join(store.root, "history", "session-1.log")
	if got := store.HistoryPath("workspace-1", "session-1"); got != want {
		t.Fatalf("HistoryPath() = %q, want %q", got, want)
	}
}

func newTestDBStore(t *testing.T) (DbStore, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Open sqlite error = %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, device_id TEXT NOT NULL, name TEXT NOT NULL, path TEXT NOT NULL, sort_order INTEGER NOT NULL DEFAULT 0, metadata_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT NULL)`,
		`CREATE INDEX idx_workspaces_device_deleted ON workspaces(device_id, deleted_at)`,
		`CREATE INDEX idx_workspaces_device_path ON workspaces(device_id, path)`,
		`CREATE INDEX idx_workspaces_device_order ON workspaces(device_id, sort_order)`,
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, device_id TEXT NOT NULL, name TEXT NOT NULL, launch_cwd TEXT NOT NULL, command_json TEXT NOT NULL, history_json TEXT NOT NULL, current_state TEXT NOT NULL, current_state_reason TEXT NOT NULL DEFAULT '', current_run_id TEXT NULL, sort_order INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT NULL, FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE)`,
		`CREATE INDEX idx_sessions_workspace_deleted ON sessions(workspace_id, deleted_at)`,
		`CREATE INDEX idx_sessions_workspace_order ON sessions(workspace_id, sort_order)`,
		`CREATE INDEX idx_sessions_device_deleted ON sessions(device_id, deleted_at)`,
		`CREATE TABLE session_runs (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, workspace_id TEXT NOT NULL, device_id TEXT NOT NULL, sequence INTEGER NOT NULL, command_json TEXT NOT NULL, terminal_size_json TEXT NOT NULL DEFAULT '{}', process_json TEXT NOT NULL DEFAULT '{}', exit_json TEXT NOT NULL DEFAULT '{}', state TEXT NOT NULL, state_reason TEXT NOT NULL DEFAULT '', started_at TEXT NULL, ended_at TEXT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT NULL, FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE, FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE)`,
		`CREATE INDEX idx_session_runs_session_sequence ON session_runs(session_id, sequence)`,
		`CREATE TABLE shortcuts (id TEXT PRIMARY KEY, device_id TEXT NOT NULL, name TEXT NOT NULL, command TEXT NOT NULL, description TEXT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE INDEX idx_shortcuts_device_updated ON shortcuts(device_id, updated_at)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(context.Background(), statement); err != nil {
			t.Fatalf("exec schema statement error = %v\n%s", err, statement)
		}
	}
	return NewDbStore(db, "sqlite", filepath.Join(t.TempDir(), ".termbridge", "devices", "device-1"), "device-1"), db
}
