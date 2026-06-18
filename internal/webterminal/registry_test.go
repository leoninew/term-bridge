package webterminal

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"termbridge-go/internal/config"
	"termbridge-go/internal/process"
	termpty "termbridge-go/internal/pty"
	"termbridge-go/internal/session"
	"termbridge-go/internal/state"
)

func TestCreateSessionPersistsRuntimeRecords(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	manager := &fakeManager{session: fake}
	registry := NewRegistry(Config{Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})

	response, err := registry.CreateSession(context.Background(), CreateSessionRequest{Command: []string{"go", "version"}, Cols: 120, Rows: 32})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if response.SessionID == "" || response.WorkspaceID == "" || response.WSURL == "" {
		t.Fatalf("response = %#v", response)
	}
	if len(manager.specs) != 1 {
		t.Fatalf("manager specs = %d, want 1", len(manager.specs))
	}
	if manager.specs[0].InitialSize.Cols != 120 || manager.specs[0].InitialSize.Rows != 32 {
		t.Fatalf("InitialSize = %#v", manager.specs[0].InitialSize)
	}
	store := state.NewStore(root)
	if _, err := store.LoadSession(response.WorkspaceKey, response.SessionID); err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	stateRecord, err := store.LoadState(response.WorkspaceKey, response.SessionID)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if stateRecord.State != session.StateRunning {
		t.Fatalf("state = %q, want running", stateRecord.State)
	}
	if _, err := store.LoadProcess(response.WorkspaceKey, response.SessionID); err != nil {
		t.Fatalf("LoadProcess() error = %v", err)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceKey, response.SessionID)
}

func TestAttachDetachAndInput(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionRequest{Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	client, err := registry.Attach(response.SessionID)
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
	summaries, err := registry.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].AttachmentState != AttachmentDetached {
		t.Fatalf("summaries = %#v", summaries)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceKey, response.SessionID)
}

func TestHistoryReturnsWrittenOutput(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), CreateSessionRequest{Command: []string{"go", "version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	fake.output <- []byte("PTY_OUTPUT\n")
	deadline := time.Now().Add(time.Second)
	for {
		data, err := os.ReadFile(state.NewStore(root).HistoryPath(response.WorkspaceKey, response.SessionID))
		if err == nil && string(data) == "PTY_OUTPUT\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("history was not written, data=%q err=%v", data, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	data, err := registry.History(response.SessionID)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if string(data) != "PTY_OUTPUT\n" {
		t.Fatalf("History() = %q", data)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceKey, response.SessionID)
}

func waitExit(t *testing.T, store state.Store, workspaceKey string, sessionID string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := store.LoadExit(workspaceKey, sessionID); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("exit record was not written for %s", sessionID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type fakeManager struct {
	session *fakeSession
	specs   []process.ProcessSpec
}

func (m *fakeManager) Start(ctx context.Context, spec process.ProcessSpec) (termpty.Session, error) {
	m.specs = append(m.specs, spec)
	return m.session, nil
}

type fakeSession struct {
	output  chan []byte
	done    chan termpty.Result
	written []byte
	closed  bool
}

func newFakeSession() *fakeSession {
	return &fakeSession{output: make(chan []byte, 8), done: make(chan termpty.Result, 1)}
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

func (s *fakeSession) Resize(size process.TerminalSize) error { return nil }
func (s *fakeSession) Interrupt() error                       { return nil }
func (s *fakeSession) Close() error                           { s.closed = true; return nil }
func (s *fakeSession) KillTree() error                        { return nil }
func (s *fakeSession) Wait() termpty.Result                   { return <-s.done }
func (s *fakeSession) ProcessInfo() process.Record {
	return process.Record{SchemaVersion: 1, PID: 123, OwnerPID: os.Getpid(), Executable: "go", CommandLine: "go version", Cwd: ".", StartedAt: time.Now().UTC()}
}
func (s *fakeSession) finish(result termpty.Result) {
	close(s.output)
	s.done <- result
}
