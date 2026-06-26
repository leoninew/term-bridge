package terminal

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"termbridge-go/internal/domain/process"
	"termbridge-go/internal/domain/session"
	"termbridge-go/internal/infrastructure/config"
	"termbridge-go/internal/infrastructure/logging"
	termpty "termbridge-go/internal/infrastructure/pty"
	"termbridge-go/internal/infrastructure/repository/state"
)

func TestCreateSessionExpandsHomeCwd(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	workdir := filepath.Join(home, "Downloads")
	if err := os.Mkdir(workdir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	fake := newFakeSession()
	manager := &fakeManager{session: fake}
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: home, Store: state.NewStore(root), LogDir: filepath.Join(home, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})

	_, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Cwd: "~/Downloads", Command: []string{"go", "version"}, Cols: 120, Rows: 32})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	want, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	if manager.specs[0].Cwd != want {
		t.Fatalf("spec cwd = %q, want %q", manager.specs[0].Cwd, want)
	}
}

func TestCreateSessionAcceptsHomeCwd(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	fake := newFakeSession()
	manager := &fakeManager{session: fake}
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: home, Store: state.NewStore(root), LogDir: filepath.Join(home, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})

	_, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Home shell", Cwd: "~", Command: []string{"go", "version"}, Cols: 120, Rows: 32})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	want, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	if manager.specs[0].Cwd != want {
		t.Fatalf("spec cwd = %q, want %q", manager.specs[0].Cwd, want)
	}
}

func TestCreateSessionPersistsRuntimeRecords(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	manager := &fakeManager{session: fake}
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})

	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}, Cols: 120, Rows: 32})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if response.SessionId == "" || response.WorkspaceId == "" {
		t.Fatalf("response = %#v", response)
	}
	if len(manager.specs) != 1 {
		t.Fatalf("manager specs = %d, want 1", len(manager.specs))
	}
	if manager.specs[0].InitialSize.Cols != 120 || manager.specs[0].InitialSize.Rows != 32 {
		t.Fatalf("InitialSize = %#v", manager.specs[0].InitialSize)
	}
	store := state.NewStore(root)
	if _, err := store.LoadSession(response.WorkspaceId, response.SessionId); err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	stateRecord, err := store.LoadState(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if stateRecord.State != session.StateRunning {
		t.Fatalf("state = %q, want running", stateRecord.State)
	}
	if _, err := store.LoadProcess(response.WorkspaceId, response.SessionId); err != nil {
		t.Fatalf("LoadProcess() error = %v", err)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestRuntimeInitialSizeMatchesCreatedPTYSize(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}, Cols: 120, Rows: 32})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	client, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if err := client.Resize(120, 32); err != nil {
		t.Fatalf("Resize() error = %v", err)
	}
	if len(fake.resizes) != 0 {
		t.Fatalf("resizes = %#v, want none for initial size", fake.resizes)
	}
	if err := client.Resize(121, 32); err != nil {
		t.Fatalf("Resize() error = %v", err)
	}
	if len(fake.resizes) != 1 || fake.resizes[0] != (process.TerminalSize{Cols: 121, Rows: 32}) {
		t.Fatalf("resizes = %#v, want 121x32", fake.resizes)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestRuntimeRetriesResizeAfterFailure(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	fake.resizeErrs = []error{errors.New("resize failed")}
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}, Cols: 120, Rows: 32})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	client, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if err := client.Resize(121, 32); err == nil {
		t.Fatal("Resize() error = nil, want failure")
	}
	if err := client.Resize(121, 32); err != nil {
		t.Fatalf("Resize() retry error = %v", err)
	}
	if len(fake.resizes) != 2 || fake.resizes[0] != (process.TerminalSize{Cols: 121, Rows: 32}) || fake.resizes[1] != (process.TerminalSize{Cols: 121, Rows: 32}) {
		t.Fatalf("resizes = %#v, want two attempts to 121x32", fake.resizes)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestAttachDetachAndInput(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	client, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if err := client.WriteInput([]byte("hello")); err != nil {
		t.Fatalf("WriteInput() error = %v", err)
	}
	if string(fake.written) != "hello" {
		t.Fatalf("written = %q", fake.written)
	}
	client.Detach("test_detach")
	waitRuntimeAttachment(t, registry, response.SessionId, AttachmentDetached)
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestHistoryReturnsWrittenOutput(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	fake.output <- []byte("PTY_OUTPUT\n")
	var data []byte
	deadline := time.Now().Add(time.Second)
	for {
		var err error
		data, err = registry.History(response.WorkspaceId, response.SessionId)
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if string(data) == "PTY_OUTPUT\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("history was not written, data=%q", data)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if string(data) != "PTY_OUTPUT\n" {
		t.Fatalf("History() = %q", data)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestSlowClientDetachDoesNotStopRuntimeOrHistory(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 4096, MaxLineBytes: 4096}, Manager: &fakeManager{session: fake}, ClientQueueSize: 4, ClientQueueBytes: 4096})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Large output", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	client, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	drainClient(t, client)
	for i := 0; i < 8; i++ {
		fake.output <- []byte("TERM_BRIDGE_LARGE_OUTPUT_" + strings.Repeat("x", 128) + "\n")
	}
	waitRuntimeAttachment(t, registry, response.SessionId, AttachmentDetached)
	data, err := registry.History(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if !strings.Contains(string(data), "TERM_BRIDGE_LARGE_OUTPUT_") {
		t.Fatalf("history missing output: %q", data)
	}
	if !registry.hasRuntime(response.SessionId) {
		t.Fatal("runtime stopped after slow client detach")
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestRuntimeSummaryOverridesStaleFailedState(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	store := state.NewStore(root)
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: store, LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := store.SaveState(response.WorkspaceId, response.SessionId, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "stale_process_unverified", UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	tree, err := registry.WorkspaceTree()
	if err != nil {
		t.Fatalf("WorkspaceTree() error = %v", err)
	}
	if got := tree[0].Children[0].LifecycleState; got != session.StateRunning {
		t.Fatalf("LifecycleState = %q, want running", got)
	}
	if err := registry.DeleteSession(response.WorkspaceId, response.SessionId); err == nil {
		t.Fatal("DeleteSession() error = nil, want running session rejection")
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, store, response.WorkspaceId, response.SessionId)
}

func TestCreateSessionRequiresName(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: newFakeSession()}})

	_, err := registry.CreateSession(context.Background(), CreateSessionReq{Command: []string{"go", "version"}})
	if err == nil {
		t.Fatal("CreateSession() error = nil, want error")
	}
}

func TestUpdateSessionChangesName(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Old name", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	summary, err := registry.UpdateSession(response.WorkspaceId, response.SessionId, UpdateSessionReq{Name: "New name"})
	if err != nil {
		t.Fatalf("UpdateSession() error = %v", err)
	}
	if summary.Name != "New name" {
		t.Fatalf("Name = %q, want New name", summary.Name)
	}
	stored, err := state.NewStore(root).LoadSession(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if stored.Name != "New name" {
		t.Fatalf("stored name = %q", stored.Name)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestUpdateSessionRejectsUnsupportedFields(t *testing.T) {
	var request UpdateSessionReq
	if err := request.UnmarshalJSON([]byte(`{"name":"ok","cwd":"/tmp"}`)); err == nil {
		t.Fatal("UnmarshalJSON() error = nil, want unsupported field error")
	}
}

func TestDeleteSessionRejectsRunningAndAllowsStopped(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := registry.DeleteSession(response.WorkspaceId, response.SessionId); err == nil {
		t.Fatal("DeleteSession() error = nil, want running session rejection")
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
	waitRuntimeRemoved(t, registry, response.SessionId)
	if err := registry.DeleteSession(response.WorkspaceId, response.SessionId); err != nil {
		t.Fatalf("DeleteSession() stopped error = %v", err)
	}
	if _, err := state.NewStore(root).LoadSession(response.WorkspaceId, response.SessionId); err == nil {
		t.Fatal("LoadSession() error = nil, want deleted session")
	}
}

func TestWorkspaceTreeAndOrder(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	tree, err := registry.WorkspaceTree()
	if err != nil {
		t.Fatalf("WorkspaceTree() error = %v", err)
	}
	if len(tree) != 1 || tree[0].Id != response.WorkspaceId || len(tree[0].Children) != 1 || tree[0].Children[0].Name != "Go version" {
		t.Fatalf("tree = %#v", tree)
	}
	workspaces, err := registry.UpdateWorkspaceOrder([]string{response.WorkspaceId})
	if err != nil {
		t.Fatalf("UpdateWorkspaceOrder() error = %v", err)
	}
	if len(workspaces) != 1 || workspaces[0].SortOrder != 1 {
		t.Fatalf("workspaces = %#v", workspaces)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestDeleteWorkspaceProtectsRunningSessions(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := registry.DeleteWorkspace(response.WorkspaceId); err == nil {
		t.Fatal("DeleteWorkspace() error = nil, want running session rejection")
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
	waitRuntimeRemoved(t, registry, response.SessionId)
	if err := registry.DeleteWorkspace(response.WorkspaceId); err != nil {
		t.Fatalf("DeleteWorkspace() stopped error = %v", err)
	}
	if _, err := state.NewStore(root).LoadWorkspace(response.WorkspaceId); err == nil {
		t.Fatal("LoadWorkspace() error = nil, want deleted workspace")
	}
}

func TestCloseSessionReturnsStoppedSummary(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	go func() {
		<-fake.closeSignal
		fake.finish(termpty.Result{ExitCode: 0})
	}()

	summary, err := registry.CloseSession(response.WorkspaceId, response.SessionId, "test_close")
	if err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
	if summary.Id != response.SessionId || summary.LifecycleState != session.StateStopped {
		t.Fatalf("summary = %#v, want stopped session %s", summary, response.SessionId)
	}
}

func TestRerunSessionArchivesHistoryAndReusesSessionId(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	first := newFakeSession()
	second := newFakeSession()
	manager := &fakeManager{sessions: []*fakeSession{first, second}}
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}, Cols: 120, Rows: 32})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	first.output <- []byte("OLD_OUTPUT\n")
	waitHistoryContains(t, registry, response.WorkspaceId, response.SessionId, "OLD_OUTPUT")
	first.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
	waitRuntimeRemoved(t, registry, response.SessionId)

	rerun, err := registry.RerunSession(context.Background(), response.WorkspaceId, response.SessionId, RerunSessionReq{Cols: 100, Rows: 30})
	if err != nil {
		t.Fatalf("RerunSession() error = %v", err)
	}
	if rerun.SessionId != response.SessionId || rerun.WorkspaceId != response.WorkspaceId || rerun.State != string(session.StateRunning) {
		t.Fatalf("rerun response = %#v", rerun)
	}
	if len(manager.specs) != 2 {
		t.Fatalf("manager specs = %d, want 2", len(manager.specs))
	}
	if manager.specs[1].Command != "go" || len(manager.specs[1].Args) != 1 || manager.specs[1].Args[0] != "version" {
		t.Fatalf("rerun spec = %#v", manager.specs[1])
	}
	if manager.specs[1].InitialSize != (process.TerminalSize{Cols: 100, Rows: 30}) {
		t.Fatalf("rerun size = %#v", manager.specs[1].InitialSize)
	}
	if data, err := registry.History(response.WorkspaceId, response.SessionId); err != nil || string(data) != "" {
		t.Fatalf("new history = %q, err=%v; want empty", data, err)
	}
	storedWorkspace, err := state.NewStore(root).LoadWorkspace(response.WorkspaceId)
	if err != nil {
		t.Fatalf("LoadWorkspace() error = %v", err)
	}
	child := storedWorkspace.Children[0]
	archiveDir := state.NewStore(root).SessionDir(response.WorkspaceId, response.SessionId)
	matches, err := filepath.Glob(filepath.Join(archiveDir, "history.*.log"))
	if err != nil {
		t.Fatalf("Glob(archive) error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("archive count = %d, want 1; files = %v", len(matches), matches)
	}
	archived, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile(archive) error = %v", err)
	}
	if string(archived) != "OLD_OUTPUT\n" {
		t.Fatalf("archive history = %q", archived)
	}
	_ = child
	assertNoArchiveFragments(t, state.NewStore(root).SessionDir(response.WorkspaceId, response.SessionId))
	second.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestRerunSessionRejectsRunningSession(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, err := registry.RerunSession(context.Background(), response.WorkspaceId, response.SessionId, RerunSessionReq{}); err == nil {
		t.Fatal("RerunSession() error = nil, want running session rejection")
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestRerunSessionFailureMarksFailed(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	first := newFakeSession()
	manager := &fakeManager{sessions: []*fakeSession{first}}
	registry := NewRegistry(Config{Logger: &logging.Logger{Slog: slog.Default()}, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})
	response, err := registry.CreateSession(context.Background(), CreateSessionReq{Name: "Go version", Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	first.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
	waitRuntimeRemoved(t, registry, response.SessionId)
	if err := os.RemoveAll(cwd); err != nil {
		t.Fatalf("RemoveAll(cwd) error = %v", err)
	}

	if _, err := registry.RerunSession(context.Background(), response.WorkspaceId, response.SessionId, RerunSessionReq{}); err == nil {
		t.Fatal("RerunSession() error = nil, want invalid cwd")
	}
	stateRecord, err := state.NewStore(root).LoadState(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if stateRecord.State != session.StateFailed {
		t.Fatalf("state = %#v, want failed", stateRecord)
	}
}

func waitHistoryContains(t *testing.T, registry *Registry, workspaceId string, sessionId string, want string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		data, err := registry.History(workspaceId, sessionId)
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if strings.Contains(string(data), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("history = %q, want substring %q", data, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertNoArchiveFragments(t *testing.T, sessionDir string) {
	t.Helper()
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "process.") || strings.HasPrefix(name, "exit.") || strings.HasPrefix(name, "state.") {
			t.Fatalf("unexpected archive fragment %s", name)
		}
	}
}

func waitExit(t *testing.T, store state.Store, workspaceKey string, sessionId string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		exitReady := false
		if _, err := store.LoadExit(workspaceKey, sessionId); err == nil {
			exitReady = true
		}
		stateRecord, stateErr := store.LoadState(workspaceKey, sessionId)
		if exitReady && stateErr == nil && stateRecord.State == session.StateStopped {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("session did not stop for %s; exitReady=%v state=%#v stateErr=%v", sessionId, exitReady, stateRecord, stateErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitRuntimeRemoved(t *testing.T, registry *Registry, sessionId string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if !registry.hasRuntime(sessionId) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("runtime was not removed for %s", sessionId)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitRuntimeAttachment(t *testing.T, registry *Registry, sessionId string, want AttachmentState) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		registry.mu.Lock()
		runtime := registry.runtimes[sessionId]
		registry.mu.Unlock()
		if runtime != nil && runtime.attachmentState() == want {
			return
		}
		if time.Now().After(deadline) {
			got := AttachmentUnattached
			if runtime != nil {
				got = runtime.attachmentState()
			}
			t.Fatalf("attachment = %q, want %q", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func drainClient(t *testing.T, client *Client) {
	t.Helper()
	for {
		select {
		case outbound, ok := <-client.Outbound():
			if !ok {
				return
			}
			client.MarkSent(outbound)
		default:
			return
		}
	}
}

type fakeManager struct {
	session  *fakeSession
	sessions []*fakeSession
	specs    []process.ProcessSpec
}

func (m *fakeManager) Start(ctx context.Context, spec process.ProcessSpec) (termpty.Session, error) {
	m.specs = append(m.specs, spec)
	if len(m.sessions) > 0 {
		session := m.sessions[0]
		m.sessions = m.sessions[1:]
		return session, nil
	}
	return m.session, nil
}

type fakeSession struct {
	output      chan []byte
	done        chan termpty.Result
	closeSignal chan struct{}
	written     []byte
	resizes     []process.TerminalSize
	resizeErrs  []error
	closed      bool
}

func newFakeSession() *fakeSession {
	return &fakeSession{output: make(chan []byte, 8), done: make(chan termpty.Result, 1), closeSignal: make(chan struct{}, 1)}
}

func (s *fakeSession) Read(p []byte) (int, error) {
	data, ok := <-s.output
	if !ok {
		return 0, io.EOF
	}
	return copy(p, data), nil
}

func (s *fakeSession) Write(p []byte) (int, error) {
	s.written = append(s.written, p...)
	return len(p), nil
}

func (s *fakeSession) Resize(size process.TerminalSize) error {
	s.resizes = append(s.resizes, size)
	if len(s.resizeErrs) > 0 {
		err := s.resizeErrs[0]
		s.resizeErrs = s.resizeErrs[1:]
		return err
	}
	return nil
}
func (s *fakeSession) Interrupt() error { return nil }
func (s *fakeSession) Close() error {
	s.closed = true
	select {
	case s.closeSignal <- struct{}{}:
	default:
	}
	return nil
}
func (s *fakeSession) KillTree() error      { return nil }
func (s *fakeSession) Wait() termpty.Result { return <-s.done }
func (s *fakeSession) ProcessInfo() process.Record {
	return process.Record{SchemaVersion: 1, Pid: 123, OwnerPid: os.Getpid(), Executable: "go", CommandLine: "go version", Cwd: ".", StartedAt: time.Now().UTC()}
}
func (s *fakeSession) finish(result termpty.Result) {
	close(s.output)
	s.done <- result
}
