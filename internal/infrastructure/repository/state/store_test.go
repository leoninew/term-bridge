package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"termbridge-go/internal/domain/process"
	"termbridge-go/internal/domain/session"
	"termbridge-go/internal/domain/workspace"
)

func TestStoreSavesAndListsRecordsFromWorkspaceAggregate(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "01", Name: "project", Path: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "02", WorkspaceId: ws.Id, Name: "shell", LaunchCwd: ws.Path, Command: session.CommandRecord{Command: "pwsh", Args: []string{"-NoLogo"}}, History: session.HistoryRecord{Path: "history.log", MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := store.SaveState(ws.Id, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "done", UpdatedAt: now}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	if err := store.SaveProcess(ws.Id, sess.Id, process.Record{SchemaVersion: 1, Pid: 123, StartedAt: now}); err != nil {
		t.Fatalf("SaveProcess() error = %v", err)
	}
	if err := store.SaveExit(ws.Id, sess.Id, process.ExitRecord{SchemaVersion: 1, ExitCode: 7, Reason: "user_process_exited", EndedAt: now}); err != nil {
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
	storedWorkspace, err := store.LoadWorkspace(ws.Id)
	if err != nil {
		t.Fatalf("LoadWorkspace() error = %v", err)
	}
	child := storedWorkspace.Children[0]
	if child.State.State != string(session.StateStopped) || child.CurrentRun.Process == nil || child.CurrentRun.Exit == nil {
		t.Fatalf("workspace child aggregate = %#v", child)
	}
	assertNotExists(t, filepath.Join(store.SessionDir(ws.Id, sess.Id), "session.json"))
	assertNotExists(t, filepath.Join(store.SessionDir(ws.Id, sess.Id), "state.json"))
	assertNotExists(t, filepath.Join(store.SessionDir(ws.Id, sess.Id), "process.json"))
	assertNotExists(t, filepath.Join(store.SessionDir(ws.Id, sess.Id), "exit.json"))
}

func TestStoreOverwritesExistingState(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	ws, sess := saveWorkspaceSession(t, store, now)
	first := session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: now}
	second := session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "user_process_exited", UpdatedAt: now.Add(time.Second)}

	if err := store.SaveState(ws.Id, sess.Id, first); err != nil {
		t.Fatalf("SaveState(first) error = %v", err)
	}
	if err := store.SaveState(ws.Id, sess.Id, second); err != nil {
		t.Fatalf("SaveState(second) error = %v", err)
	}
	stored, err := store.LoadState(ws.Id, sess.Id)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if stored.State != session.StateStopped || stored.Reason != "user_process_exited" {
		t.Fatalf("state = %#v, want stopped user_process_exited", stored)
	}
}

func TestStoreIgnoresLegacySessionFragments(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	ws, sess := saveWorkspaceSession(t, store, now)
	if err := store.SaveState(ws.Id, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "workspace_state", UpdatedAt: now}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	if err := store.SaveProcess(ws.Id, sess.Id, process.Record{SchemaVersion: 1, Pid: 123}); err != nil {
		t.Fatalf("SaveProcess() error = %v", err)
	}
	if err := store.SaveExit(ws.Id, sess.Id, process.ExitRecord{SchemaVersion: 1, ExitCode: 7, Reason: "workspace_exit"}); err != nil {
		t.Fatalf("SaveExit() error = %v", err)
	}
	legacyDir := store.SessionDir(ws.Id, sess.Id)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	legacyFiles := map[string]string{
		"session.json": `{"session_id":"legacy","name":"legacy"}`,
		"state.json":   `{"state":"running","reason":"legacy_state"}`,
		"process.json": `{"pid":999}`,
		"exit.json":    `{"exit_code":99,"reason":"legacy_exit"}`,
	}
	for name, body := range legacyFiles {
		if err := os.WriteFile(filepath.Join(legacyDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

	stateRecord, err := store.LoadState(ws.Id, sess.Id)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if stateRecord.State != session.StateStopped || stateRecord.Reason != "workspace_state" {
		t.Fatalf("state = %#v, want workspace aggregate", stateRecord)
	}
	processRecord, err := store.LoadProcess(ws.Id, sess.Id)
	if err != nil {
		t.Fatalf("LoadProcess() error = %v", err)
	}
	if processRecord.Pid != 123 {
		t.Fatalf("process = %#v, want workspace aggregate", processRecord)
	}
	exitRecord, err := store.LoadExit(ws.Id, sess.Id)
	if err != nil {
		t.Fatalf("LoadExit() error = %v", err)
	}
	if exitRecord.ExitCode != 7 || exitRecord.Reason != "workspace_exit" {
		t.Fatalf("exit = %#v, want workspace aggregate", exitRecord)
	}
}

func TestArchiveCurrentRunMovesCurrentMetadataIntoWorkspaceArchive(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	ws, sess := saveWorkspaceSession(t, store, now)
	if err := store.SaveState(ws.Id, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "done", UpdatedAt: now}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	if err := store.SaveProcess(ws.Id, sess.Id, process.Record{SchemaVersion: 1, Pid: 123}); err != nil {
		t.Fatalf("SaveProcess() error = %v", err)
	}
	if err := store.SaveExit(ws.Id, sess.Id, process.ExitRecord{SchemaVersion: 1, ExitCode: 7}); err != nil {
		t.Fatalf("SaveExit() error = %v", err)
	}

	archivedAt := now.Add(time.Minute)
	if err := store.ArchiveCurrentRun(ws.Id, sess.Id, "20260618T100100Z", "history.20260618T100100Z.log", archivedAt); err != nil {
		t.Fatalf("ArchiveCurrentRun() error = %v", err)
	}
	storedWorkspace, err := store.LoadWorkspace(ws.Id)
	if err != nil {
		t.Fatalf("LoadWorkspace() error = %v", err)
	}
	child := storedWorkspace.Children[0]
	if child.CurrentRun.Process != nil || child.CurrentRun.Exit != nil {
		t.Fatalf("current run = %#v, want cleared", child.CurrentRun)
	}
	if len(child.ArchivedRuns) != 1 {
		t.Fatalf("archived runs = %#v", child.ArchivedRuns)
	}
	archived := child.ArchivedRuns[0]
	if archived.State.State != string(session.StateStopped) || archived.Process == nil || archived.Process.Pid != 123 || archived.Exit == nil || archived.Exit.ExitCode != 7 {
		t.Fatalf("archived run = %#v", archived)
	}
}

func TestStoreOrdersAndDeletesWorkspaceSessions(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	first := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "first", Name: "first", Path: t.TempDir(), SortOrder: 2, CreatedAt: now, UpdatedAt: now}
	second := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "second", Name: "second", Path: t.TempDir(), SortOrder: 1, CreatedAt: now, UpdatedAt: now}
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
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "session", Name: "Session", WorkspaceId: second.Id, LaunchCwd: second.Path, Command: session.CommandRecord{Command: "zsh"}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	storedWorkspace, err := store.LoadWorkspace(second.Id)
	if err != nil {
		t.Fatalf("LoadWorkspace() error = %v", err)
	}
	if len(storedWorkspace.Children) != 1 || storedWorkspace.Children[0].Name != "Session" {
		t.Fatalf("workspace children = %#v", storedWorkspace.Children)
	}
	if err := store.SaveState(second.Id, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	views, _, err := store.ListSessionsByWorkspaceId(second.Id)
	if err != nil {
		t.Fatalf("ListSessionsByWorkspaceId() error = %v", err)
	}
	if len(views) != 1 || views[0].Session.Name != "Session" {
		t.Fatalf("views = %#v", views)
	}
	if err := store.DeleteSession(second.Id, sess.Id); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if _, err := store.LoadSession(second.Id, sess.Id); err == nil {
		t.Fatal("LoadSession() error = nil, want deleted session")
	}
	storedWorkspace, err = store.LoadWorkspace(second.Id)
	if err != nil {
		t.Fatalf("LoadWorkspace() after delete session error = %v", err)
	}
	if len(storedWorkspace.Children) != 0 {
		t.Fatalf("workspace children after delete = %#v", storedWorkspace.Children)
	}
	if err := store.DeleteWorkspace(second.Id); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v", err)
	}
	if _, err := store.LoadWorkspace(second.Id); err == nil {
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

func TestFindWorkspaceByIdUsesWorkspaceIdDirectory(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), ".termbridge"))
	workspaceId := "workspace-id"
	wrongDir := filepath.Join(store.WorkspaceRoot(), "old-key")
	if err := os.MkdirAll(wrongDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	value := []byte(`{
  "schema_version": 1,
  "workspace_id": "workspace-id",
  "name": "project",
  "path": ".",
  "children": [],
  "created_at": "2026-06-23T00:00:00Z",
  "updated_at": "2026-06-23T00:00:00Z"
}
`)
	if err := os.WriteFile(filepath.Join(wrongDir, "workspace.json"), value, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := store.FindWorkspaceById(workspaceId); err == nil {
		t.Fatal("FindWorkspaceById() error = nil, want missing workspace id directory")
	}
	if err := store.SaveWorkspace(workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: workspaceId, Name: "project", Path: "."}); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	got, err := store.FindWorkspaceById(workspaceId)
	if err != nil {
		t.Fatalf("FindWorkspaceById() error = %v", err)
	}
	if got.Id != workspaceId {
		t.Fatalf("FindWorkspaceById() = %#v, want workspace id %q", got, workspaceId)
	}
}

func saveWorkspaceSession(t *testing.T, store Store, now time.Time) (workspace.Workspace, session.Session) {
	t.Helper()
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace", Name: "workspace", Path: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	sess := session.Session{SchemaVersion: session.SchemaVersion, Id: "session", Name: "session", WorkspaceId: ws.Id, LaunchCwd: ws.Path, Command: session.CommandRecord{Command: "pwsh"}, History: session.HistoryRecord{Path: "history.log"}, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	return ws, sess
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s exists or stat failed with unexpected error: %v", path, err)
	}
}
