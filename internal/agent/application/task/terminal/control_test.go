package terminal

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/storage/history"
	"gitee.com/leoninew/TermBridge-go/internal/agent/repository/task/state"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
)

func TestMultiAttachControllerAndTakeover(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	registry := NewRegistry(Config{
		Logger:  slog.Default(),
		Cwd:     cwd,
		Store:   state.NewStore(root),
		LogDir:  filepath.Join(cwd, "logs"),
		History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256},
		Manager: &fakeManager{session: newFakeSession()},
	})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{
		Name:    "Shell",
		Command: []string{"bash"},
		Cols:    80,
		Rows:    24,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	first, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("first Attach() error = %v", err)
	}
	go drainControlClient(first)
	if !first.IsController() {
		t.Fatal("first client should be controller")
	}

	second, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("second Attach() error = %v", err)
	}
	go drainControlClient(second)
	if second.IsController() {
		t.Fatal("second client should be observer")
	}
	if err := second.WriteInput([]byte("x")); !IsNotController(err) {
		t.Fatalf("observer WriteInput() error = %v, want not_controller", err)
	}
	if err := second.Resize(100, 30); err != nil {
		t.Fatalf("observer Resize() error = %v, want silent nil", err)
	}

	second.TakeControl()
	waitController(t, second, true)
	waitController(t, first, false)
	if err := second.WriteInput([]byte("ok")); err != nil {
		t.Fatalf("controller WriteInput() error = %v", err)
	}

	// started should expose control_role
	// (best-effort; may already be drained)
}

func TestControllerDetachAutoPromotesOldest(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	registry := NewRegistry(Config{
		Logger:  slog.Default(),
		Cwd:     cwd,
		Store:   state.NewStore(root),
		LogDir:  filepath.Join(cwd, "logs"),
		History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256},
		Manager: &fakeManager{session: newFakeSession()},
	})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{
		Name:    "Shell",
		Command: []string{"bash"},
		Cols:    80,
		Rows:    24,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	first, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("first Attach() error = %v", err)
	}
	second, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("second Attach() error = %v", err)
	}
	third, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("third Attach() error = %v", err)
	}
	go drainControlClient(first)
	go drainControlClient(second)
	go drainControlClient(third)

	if !first.IsController() || second.IsController() || third.IsController() {
		t.Fatalf("initial roles first=%v second=%v third=%v", first.IsController(), second.IsController(), third.IsController())
	}

	first.Detach("controller_left")
	waitController(t, second, true)
	waitController(t, third, false)
}

func drainControlClient(client *Client) {
	for range client.Outbound() {
	}
}

func waitController(t *testing.T, client *Client, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if client.IsController() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("IsController() = %v, want %v", client.IsController(), want)
}

func TestWriteInputSerializesWithTakeControl(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	fake.writeStarted = make(chan struct{})
	fake.writeRelease = make(chan struct{})
	registry := NewRegistry(Config{
		Logger:  slog.Default(),
		Cwd:     cwd,
		Store:   state.NewStore(root),
		LogDir:  filepath.Join(cwd, "logs"),
		History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256},
		Manager: &fakeManager{session: fake},
	})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{
		Name:    "Shell",
		Command: []string{"bash"},
		Cols:    80,
		Rows:    24,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	first, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("first Attach() error = %v", err)
	}
	second, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("second Attach() error = %v", err)
	}
	go drainControlClient(first)
	go drainControlClient(second)

	writeDone := make(chan error, 1)
	go func() {
		writeDone <- first.WriteInput([]byte("A"))
	}()

	select {
	case <-fake.writeStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Write to start")
	}

	takeoverDone := make(chan struct{})
	go func() {
		second.TakeControl()
		close(takeoverDone)
	}()

	// TakeControl must not finish while the controller-gated write still holds controlMu.
	select {
	case <-takeoverDone:
		t.Fatal("TakeControl completed while WriteInput still in progress (TOCTOU)")
	case <-time.After(50 * time.Millisecond):
	}

	close(fake.writeRelease)

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("WriteInput() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WriteInput")
	}

	select {
	case <-takeoverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TakeControl after write")
	}

	waitController(t, second, true)
	waitController(t, first, false)
	if got := string(fake.writtenCopy()); got != "A" {
		t.Fatalf("written = %q, want %q", got, "A")
	}
	if err := first.WriteInput([]byte("X")); !IsNotController(err) {
		t.Fatalf("demoted controller WriteInput() error = %v, want not_controller", err)
	}
	if err := second.WriteInput([]byte("B")); err != nil {
		t.Fatalf("new controller WriteInput() error = %v", err)
	}
	if got := string(fake.writtenCopy()); got != "AB" {
		t.Fatalf("written = %q, want %q", got, "AB")
	}
}

func TestControlRoleNotifyQueueFullIsBestEffort(t *testing.T) {
	// Empty-history attach enqueues 3 control frames. With ClientQueueSize=3 and no drain,
	// auto-granted notify is dropped but second remains authoritative controller.
	root := t.TempDir()
	cwd := t.TempDir()
	registry := NewRegistry(Config{
		Logger:           slog.Default(),
		Cwd:              cwd,
		Store:            state.NewStore(root),
		LogDir:           filepath.Join(cwd, "logs"),
		History:          history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256},
		Manager:          &fakeManager{session: newFakeSession()},
		ClientQueueSize:  3,
		ClientQueueBytes: 1024 * 1024,
	})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{
		Name:    "Shell",
		Command: []string{"bash"},
		Cols:    80,
		Rows:    24,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	first, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("first Attach() error = %v", err)
	}
	go drainControlClient(first)

	second, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("second Attach() error = %v", err)
	}
	// Do not drain second: role notify after promote will drop.

	if !first.IsController() || second.IsController() {
		t.Fatalf("initial roles first=%v second=%v", first.IsController(), second.IsController())
	}

	first.Detach("controller_left")
	waitController(t, second, true)

	// Server authority still allows second to write despite dropped UI notify.
	if err := second.WriteInput([]byte("ok")); err != nil {
		t.Fatalf("promoted controller WriteInput() error = %v", err)
	}
}

func TestTakeControlGrantNotifyFailureKeepsNewController(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	registry := NewRegistry(Config{
		Logger:           slog.Default(),
		Cwd:              cwd,
		Store:            state.NewStore(root),
		LogDir:           filepath.Join(cwd, "logs"),
		History:          history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256},
		Manager:          &fakeManager{session: newFakeSession()},
		ClientQueueSize:  3,
		ClientQueueBytes: 1024 * 1024,
	})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{
		Name:    "Shell",
		Command: []string{"bash"},
		Cols:    80,
		Rows:    24,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	first, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("first Attach() error = %v", err)
	}
	go drainControlClient(first)

	second, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("second Attach() error = %v", err)
	}
	// second queue full (no drain).

	second.TakeControl()
	waitController(t, second, true)
	waitController(t, first, false)
	if err := second.WriteInput([]byte("ok")); err != nil {
		t.Fatalf("new controller WriteInput() error = %v", err)
	}
	if err := first.WriteInput([]byte("x")); !IsNotController(err) {
		t.Fatalf("old controller WriteInput() error = %v, want not_controller", err)
	}
}
