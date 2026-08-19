package terminal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	termpty "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/pty"
	"gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/storage/history"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/session"
	shortcutmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/shortcut"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
	"gitee.com/leoninew/TermBridge-go/internal/agent/repository/task/state"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
)

func TestRegistryUsesBoundedReplayDefaults(t *testing.T) {
	registry := NewRegistry(Config{})
	if registry.replayMaxBytes != 64*1024 {
		t.Fatalf("replayMaxBytes = %d, want 65536", registry.replayMaxBytes)
	}
	if registry.replayChunkBytes != 64*1024 {
		t.Fatalf("replayChunkBytes = %d, want 65536", registry.replayChunkBytes)
	}
}

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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: home, Store: state.NewStore(root), LogDir: filepath.Join(home, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})

	_, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Cwd: "~/Downloads", Command: []string{"go version"}, Cols: 120, Rows: 32})
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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: home, Store: state.NewStore(root), LogDir: filepath.Join(home, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})

	_, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Home shell", Cwd: "~", Command: []string{"go version"}, Cols: 120, Rows: 32})
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

func TestCreateSessionPersistsCommandSourceSnapshot(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), ShortcutStore: fakeShortcutStore{values: []shortcutmodel.Shortcut{{Id: "shortcut-1", Enabled: boolPointer(true)}}}, LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: newFakeSession()}})

	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{
		Name:                 "Review",
		Command:              []string{"codex review"},
		CommandSource:        string(session.CommandSourceShortcut),
		ShortcutIdSnapshot:   "shortcut-1",
		ShortcutNameSnapshot: "Review shortcut",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	stored, err := state.NewStore(root).LoadSession(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if stored.Command.Source != session.CommandSourceShortcut || stored.Command.ShortcutIdSnapshot != "shortcut-1" || stored.Command.ShortcutNameSnapshot != "Review shortcut" {
		t.Fatalf("stored command source = %#v, want shortcut snapshot", stored.Command)
	}
}

func TestCreateSessionRejectsDisabledShortcut(t *testing.T) {
	cwd := t.TempDir()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(t.TempDir()), ShortcutStore: fakeShortcutStore{values: []shortcutmodel.Shortcut{{Id: "shortcut-1", Enabled: boolPointer(false)}}}, LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: newFakeSession()}})

	_, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{
		Name:                 "Disabled",
		Command:              []string{"go version"},
		CommandSource:        string(session.CommandSourceShortcut),
		ShortcutIdSnapshot:   "shortcut-1",
		ShortcutNameSnapshot: "Disabled shortcut",
	})
	if err == nil || shortcutmodel.CodeOf(err) != shortcutmodel.CodeDisabled {
		t.Fatalf("CreateSession() error = %v, want shortcut_disabled", err)
	}
}

func TestCreateSessionRejectsMissingShortcut(t *testing.T) {
	cwd := t.TempDir()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(t.TempDir()), ShortcutStore: fakeShortcutStore{}, LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: newFakeSession()}})

	_, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{
		Name:                 "Missing",
		Command:              []string{"go version"},
		CommandSource:        string(session.CommandSourceShortcut),
		ShortcutIdSnapshot:   "shortcut-1",
		ShortcutNameSnapshot: "Missing shortcut",
	})
	if err == nil || shortcutmodel.CodeOf(err) != shortcutmodel.CodeNotFound {
		t.Fatalf("CreateSession() error = %v, want shortcut_not_found", err)
	}
}

func TestCreateSessionRejectsInvalidCommandSourceSnapshot(t *testing.T) {
	cwd := t.TempDir()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(t.TempDir()), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: newFakeSession()}})

	_, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{
		Name:          "Invalid",
		Command:       []string{"go version"},
		CommandSource: string(session.CommandSourceShortcut),
	})
	if err == nil || session.CodeOf(err) != session.CodeInvalidCommandSource {
		t.Fatalf("CreateSession() error = %v, want invalid_session_command_source", err)
	}
}

func TestCreateSessionPersistsRuntimeRecords(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	manager := &fakeManager{session: fake}
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})

	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}, Cols: 120, Rows: 32})
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

func TestRuntimeNonZeroProcessExitIsStoppedWithoutForcedClose(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})

	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Shell", Command: []string{"bash"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	fake.finish(termpty.Result{ExitCode: 1, Err: errors.New("exit status 1")})
	store := state.NewStore(root)
	waitExit(t, store, response.WorkspaceId, response.SessionId)

	exit, err := store.LoadExit(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("LoadExit() error = %v", err)
	}
	if exit.ExitCode != 1 || exit.Forced || exit.Closed {
		t.Fatalf("exit = %#v, want non-forced exit code 1", exit)
	}
	if fake.closed {
		t.Fatal("PTY Close() was called for non-zero process exit")
	}
}

func TestRuntimeInitialSizeMatchesCreatedPTYSize(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}, Cols: 120, Rows: 32})
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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}, Cols: 120, Rows: 32})
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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 4096, MaxLineBytes: 4096}, Manager: &fakeManager{session: fake}, ClientQueueSize: 4, ClientQueueBytes: 4096})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Large output", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	client, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	drainClient(t, client)
	for range 8 {
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

func TestAttachReplaysBoundedChunkedHistory(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 4096, MaxLineBytes: 4096}, Manager: &fakeManager{session: fake}, ClientQueueSize: 16, ClientQueueBytes: 4096, ReplayMaxBytes: 1024, ReplayChunkBytes: 256})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Replay tail", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	fake.output <- []byte("old-" + strings.Repeat("x", 2048) + "recent-" + strings.Repeat("y", 1024) + "\n")
	for {
		data, err := registry.History(response.WorkspaceId, response.SessionId)
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if strings.Contains(string(data), "recent-") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	client, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	var binaryChunks int
	var binaryBytes int
	var truncated bool
	var outboundTypes []string
	for {
		select {
		case outbound := <-client.Outbound():
			if outbound.Kind == OutboundText {
				outboundTypes = append(outboundTypes, outbound.Text.GetType())
			} else {
				outboundTypes = append(outboundTypes, "binary")
			}
			if outbound.Kind == OutboundBinary {
				binaryChunks++
				binaryBytes += len(outbound.Binary)
				if len(outbound.Binary) > 256 {
					t.Fatalf("replay chunk bytes = %d, want <= 256", len(outbound.Binary))
				}
			}
			if outbound.Text != nil && outbound.Text.Type == terminalproto.TypeReplayFinished {
				truncated = outbound.Text.GetTruncated()
				client.MarkSent(outbound)
				goto replayDone
			}
			client.MarkSent(outbound)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for replay finish")
		}
	}

replayDone:
	if !truncated {
		t.Fatal("replay truncated = false, want true")
	}
	if binaryChunks != 4 || binaryBytes != 1024 {
		t.Fatalf("replay chunks/bytes = %d/%d, want 4/1024", binaryChunks, binaryBytes)
	}
	wantOutboundTypes := []string{terminalproto.TypeStarted, terminalproto.TypeReplayStarted, "binary", "binary", "binary", "binary", terminalproto.TypeReplayFinished}
	if !slices.Equal(outboundTypes, wantOutboundTypes) {
		t.Fatalf("replay outbound sequence = %v, want %v", outboundTypes, wantOutboundTypes)
	}
	if client.QueuedBytes() != 0 {
		t.Fatalf("QueuedBytes() = %d, want released", client.QueuedBytes())
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestAttachQueuesReplayBeforeLaterLiveOutput(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{
		Logger:           slog.Default(),
		Cwd:              cwd,
		Store:            state.NewStore(root),
		LogDir:           filepath.Join(cwd, "logs"),
		History:          history.Config{MaxLines: 10, MaxBytes: 4096, MaxLineBytes: 4096},
		Manager:          &fakeManager{session: fake},
		ClientQueueSize:  16,
		ClientQueueBytes: 4096,
		ReplayMaxBytes:   1024,
		ReplayChunkBytes: 1024,
	})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Replay then live", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	fake.output <- []byte("REPLAY_OUTPUT\n")
	waitHistoryContains(t, registry, response.WorkspaceId, response.SessionId, "REPLAY_OUTPUT")

	client, err := registry.Attach(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	fake.output <- []byte("LIVE_OUTPUT\n")

	var frames []string
	deadline := time.After(time.Second)
	for {
		select {
		case outbound := <-client.Outbound():
			if outbound.Kind == OutboundText {
				frames = append(frames, outbound.Text.GetType())
			} else {
				frames = append(frames, string(outbound.Binary))
			}
			client.MarkSent(outbound)
			if outbound.Kind == OutboundBinary && string(outbound.Binary) == "LIVE_OUTPUT\n" {
				want := []string{
					terminalproto.TypeStarted,
					terminalproto.TypeReplayStarted,
					"REPLAY_OUTPUT\n",
					terminalproto.TypeReplayFinished,
					"LIVE_OUTPUT\n",
				}
				if !slices.Equal(frames, want) {
					t.Fatalf("outbound frames = %q, want %q", frames, want)
				}
				fake.finish(termpty.Result{ExitCode: 0})
				waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for live output; frames = %q", frames)
		}
	}
}

func TestAttachReplayFailureDoesNotPublishClient(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{
		Logger:           slog.Default(),
		Cwd:              cwd,
		Store:            state.NewStore(root),
		LogDir:           filepath.Join(cwd, "logs"),
		History:          history.Config{MaxLines: 10, MaxBytes: 4096, MaxLineBytes: 4096},
		Manager:          &fakeManager{session: fake},
		ClientQueueSize:  2,
		ClientQueueBytes: 4096,
		ReplayMaxBytes:   1024,
		ReplayChunkBytes: 1024,
	})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Replay queue failure", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if _, err := registry.Attach(response.WorkspaceId, response.SessionId); err == nil {
		t.Fatal("Attach() error = nil, want replay queue failure")
	}

	registry.mu.Lock()
	runtime := registry.runtimes[response.SessionId]
	registry.mu.Unlock()
	if runtime == nil {
		t.Fatal("runtime missing after replay queue failure")
	}
	runtime.mu.Lock()
	clientCount := len(runtime.clients)
	controllerClientID := runtime.controllerClientId
	attachment := runtime.attachment
	runtime.mu.Unlock()
	if clientCount != 0 || controllerClientID != "" || attachment != AttachmentDetached {
		t.Fatalf("runtime after failed attach: clients=%d controller=%q attachment=%q", clientCount, controllerClientID, attachment)
	}

	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestRuntimeSummaryOverridesStaleFailedState(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	store := state.NewStore(root)
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: store, LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
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
	if got := tree[0].Children[0].LifecycleState; got != string(session.StateRunning) {
		t.Fatalf("LifecycleState = %q, want running", got)
	}
	if err := registry.DeleteSession(response.WorkspaceId, response.SessionId); err == nil {
		t.Fatal("DeleteSession() error = nil, want running session rejection")
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, store, response.WorkspaceId, response.SessionId)
}

func TestCreateSessionPreservesRawCommandText(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	manager := &fakeManager{session: fake}
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})
	commandText := `ccs list --filter "my project"`

	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Filtered sessions", Command: []string{commandText}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if len(manager.specs) != 1 || manager.specs[0].CommandText != commandText {
		t.Fatalf("manager specs = %#v, want preserved command text", manager.specs)
	}
	stored, err := state.NewStore(root).LoadSession(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if stored.Command.Command != commandText {
		t.Fatalf("stored command = %q, want %q", stored.Command.Command, commandText)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestCreateSessionStartFailureDoesNotLogCommandText(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	const commandText = "private-token-7f3a"
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	manager := &fakeManager{err: errors.New(commandText + " executable not found")}
	registry := NewRegistry(Config{Logger: logger, Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})

	_, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Private command", Command: []string{commandText}})
	if err == nil {
		t.Fatal("CreateSession() error = nil, want startup error")
	}
	if strings.Contains(logBuffer.String(), commandText) {
		t.Fatalf("log output exposed command text: %s", logBuffer.String())
	}
}

func TestCreateSessionRejectsMultipleCommandElements(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: newFakeSession()}})

	_, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Split command", Command: []string{"ccs", "list"}})
	if err == nil {
		t.Fatal("CreateSession() error = nil, want invalid command shape")
	}
	if session.CodeOf(err) != session.CodeInvalidCommandShape {
		t.Fatalf("CreateSession() error = %v, want invalid_session_command_shape", err)
	}
}

func TestCreateSessionRequiresName(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: newFakeSession()}})

	_, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Command: []string{"go version"}})
	if err == nil {
		t.Fatal("CreateSession() error = nil, want error")
	}
}

func TestUpdateTerminalSessionChangesNameAndPreservesRawCommand(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), ShortcutStore: fakeShortcutStore{values: []shortcutmodel.Shortcut{{Id: "shortcut-1", Enabled: boolPointer(true)}}}, LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Old name", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	runningName := "Running name"
	if summary, err := registry.UpdateSession(response.WorkspaceId, response.SessionId, &agent.UpdateSessionReq{Name: &runningName}); err != nil {
		t.Fatalf("UpdateSession(name while running) error = %v", err)
	} else if summary.Name != runningName {
		t.Fatalf("UpdateSession(name while running) name = %q, want %q", summary.Name, runningName)
	}
	commandText := `ccs run c1 --prompt "review changes"`
	if _, err := registry.UpdateSession(response.WorkspaceId, response.SessionId, &agent.UpdateSessionReq{Command: &commandText}); err == nil {
		t.Fatal("UpdateSession(command while running) error = nil, want rejection")
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
	waitRuntimeRemoved(t, registry, response.SessionId)

	newName := "New name"
	commandSource := string(session.CommandSourceShortcut)
	shortcutIdSnapshot := "shortcut-1"
	shortcutNameSnapshot := "Review shortcut"
	summary, err := registry.UpdateSession(response.WorkspaceId, response.SessionId, &agent.UpdateSessionReq{Name: &newName, Command: &commandText, CommandSource: &commandSource, ShortcutIdSnapshot: &shortcutIdSnapshot, ShortcutNameSnapshot: &shortcutNameSnapshot})
	if err != nil {
		t.Fatalf("UpdateSession() error = %v", err)
	}
	if summary.Name != newName || summary.Command != commandText || summary.Cwd != cwd || summary.CommandSource != commandSource || summary.ShortcutIdSnapshot != shortcutIdSnapshot || summary.ShortcutNameSnapshot != shortcutNameSnapshot {
		t.Fatalf("UpdateSession() summary = %#v, want updated name/raw command/source and unchanged cwd", summary)
	}
	stored, err := state.NewStore(root).LoadSession(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if stored.Name != newName || stored.Command.Command != commandText || stored.Command.Source != session.CommandSourceShortcut || stored.Command.ShortcutIdSnapshot != shortcutIdSnapshot || stored.Command.ShortcutNameSnapshot != shortcutNameSnapshot || stored.LaunchCwd != cwd {
		t.Fatalf("stored session = %#v, want updated name/raw command/source and unchanged cwd", stored)
	}
}

func TestUpdateSessionRejectsDisabledShortcut(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), ShortcutStore: fakeShortcutStore{values: []shortcutmodel.Shortcut{{Id: "shortcut-1", Enabled: boolPointer(false)}}}, LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	created, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Shell", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), created.WorkspaceId, created.SessionId)

	command := "go test ./..."
	commandSource := string(session.CommandSourceShortcut)
	shortcutId := "shortcut-1"
	_, err = registry.UpdateSession(created.WorkspaceId, created.SessionId, &agent.UpdateSessionReq{Command: &command, CommandSource: &commandSource, ShortcutIdSnapshot: &shortcutId})
	if err == nil || shortcutmodel.CodeOf(err) != shortcutmodel.CodeDisabled {
		t.Fatalf("UpdateSession() error = %v, want shortcut_disabled", err)
	}
	stored, err := state.NewStore(root).LoadSession(created.WorkspaceId, created.SessionId)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if stored.Command.Command != "go version" || stored.Command.Source != "" || stored.Command.ShortcutIdSnapshot != "" {
		t.Fatalf("stored command = %#v, want original direct command", stored.Command)
	}
}

func TestUpdateSessionAllowsFailedSessionAndRejectsEmptyPatch(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	store := state.NewStore(root)
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: store, LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Old name", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, err := registry.UpdateSession(response.WorkspaceId, response.SessionId, &agent.UpdateSessionReq{}); err == nil {
		t.Fatal("UpdateSession(empty patch) error = nil, want rejection")
	}
	if err := store.SaveState(response.WorkspaceId, response.SessionId, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "launch_failed", UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("SaveState(failed) error = %v", err)
	}
	newName := "New name"
	updated, err := registry.UpdateSession(response.WorkspaceId, response.SessionId, &agent.UpdateSessionReq{Name: &newName})
	if err != nil {
		t.Fatalf("UpdateSession(failed) error = %v", err)
	}
	if updated.Name != newName || updated.Cwd != cwd {
		t.Fatalf("UpdateSession(failed) = %#v, want updated name and unchanged cwd", updated)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, store, response.WorkspaceId, response.SessionId)
}

func boolPointer(value bool) *bool {
	return &value
}

type fakeShortcutStore struct {
	values []shortcutmodel.Shortcut
	err    error
}

func (s fakeShortcutStore) ListShortcuts() ([]shortcutmodel.Shortcut, error) {
	return s.values, s.err
}

func TestDeleteSessionRejectsRunningAndAllowsStopped(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
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
	if len(workspaces) != 1 || workspaces[0].Id != response.WorkspaceId {
		t.Fatalf("workspaces = %#v", workspaces)
	}
	fake.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
}

func TestRegistryOrdersWorkspaceSessions(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	store := state.NewStore(root)
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	ws := workspace.Workspace{SchemaVersion: workspace.SchemaVersion, Id: "workspace", Name: "Workspace", Path: cwd, CreatedAt: now, UpdatedAt: now}
	if err := store.SaveWorkspace(ws); err != nil {
		t.Fatalf("SaveWorkspace() error = %v", err)
	}
	first := session.Session{SchemaVersion: session.SchemaVersion, Id: "session-1", WorkspaceId: ws.Id, Name: "First", LaunchCwd: cwd, Command: session.CommandRecord{Command: "go version"}, CreatedAt: now, UpdatedAt: now}
	second := session.Session{SchemaVersion: session.SchemaVersion, Id: "session-2", WorkspaceId: ws.Id, Name: "Second", LaunchCwd: cwd, Command: session.CommandRecord{Command: "go version"}, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)}
	for _, sess := range []session.Session{first, second} {
		if err := store.SaveSession(sess); err != nil {
			t.Fatalf("SaveSession() error = %v", err)
		}
		if err := store.SaveState(ws.Id, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, UpdatedAt: now}); err != nil {
			t.Fatalf("SaveState() error = %v", err)
		}
	}
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: store, LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: newFakeSession()}})
	ordered, err := registry.UpdateSessionOrder(ws.Id, []string{second.Id, first.Id})
	if err != nil {
		t.Fatalf("UpdateSessionOrder() error = %v", err)
	}
	if len(ordered) != 2 || ordered[0].Id != second.Id || ordered[1].Id != first.Id {
		t.Fatalf("ordered = %#v", ordered)
	}
	tree, err := registry.WorkspaceTree()
	if err != nil {
		t.Fatalf("WorkspaceTree() error = %v", err)
	}
	if len(tree) != 1 || len(tree[0].Children) != 2 || tree[0].Children[0].Id != second.Id || tree[0].Children[1].Id != first.Id {
		t.Fatalf("tree = %#v", tree)
	}
}

func TestDeleteWorkspaceProtectsRunningSessions(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
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
	if summary.Id != response.SessionId || summary.LifecycleState != string(session.StateStopped) {
		t.Fatalf("summary = %#v, want stopped session %s", summary, response.SessionId)
	}
}

func TestCloseSessionMissingRuntimeWarnsAndReturnsStoppedSummary(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	fake := newFakeSession()
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	registry.removeRuntime(response.SessionId)

	summary, err := registry.CloseSession(response.WorkspaceId, response.SessionId, "test_close")
	if err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
	if summary.Id != response.SessionId || summary.LifecycleState != string(session.StateStopped) {
		t.Fatalf("summary = %#v, want stopped session %s", summary, response.SessionId)
	}
	stateRecord, err := state.NewStore(root).LoadState(response.WorkspaceId, response.SessionId)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if stateRecord.State != session.StateStopped || stateRecord.Reason != "missing_pty_on_close" {
		t.Fatalf("state = %#v, want stopped missing_pty_on_close", stateRecord)
	}
}

func TestRerunSessionArchivesHistoryAndReusesSessionId(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	first := newFakeSession()
	second := newFakeSession()
	manager := &fakeManager{sessions: []*fakeSession{first, second}}
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}, Cols: 120, Rows: 32})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	first.output <- []byte("OLD_OUTPUT\n")
	waitHistoryContains(t, registry, response.WorkspaceId, response.SessionId, "OLD_OUTPUT")
	first.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
	waitRuntimeRemoved(t, registry, response.SessionId)

	rerun, err := registry.RerunSession(context.Background(), response.WorkspaceId, response.SessionId, &agent.RerunSessionReq{Cols: 100, Rows: 30})
	if err != nil {
		t.Fatalf("RerunSession() error = %v", err)
	}
	if rerun.SessionId != response.SessionId || rerun.WorkspaceId != response.WorkspaceId || rerun.State != string(session.StateRunning) {
		t.Fatalf("rerun response = %#v", rerun)
	}
	if len(manager.specs) != 2 {
		t.Fatalf("manager specs = %d, want 2", len(manager.specs))
	}
	if manager.specs[1].CommandText != "go version" {
		t.Fatalf("rerun command text = %q, want %q", manager.specs[1].CommandText, "go version")
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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: &fakeManager{session: fake}})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, err := registry.RerunSession(context.Background(), response.WorkspaceId, response.SessionId, &agent.RerunSessionReq{}); err == nil {
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
	registry := NewRegistry(Config{Logger: slog.Default(), Cwd: cwd, Store: state.NewStore(root), LogDir: filepath.Join(cwd, "logs"), History: history.Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}, Manager: manager})
	response, err := registry.CreateSession(context.Background(), &agent.CreateSessionReq{Name: "Go version", Command: []string{"go version"}})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	first.finish(termpty.Result{ExitCode: 0})
	waitExit(t, state.NewStore(root), response.WorkspaceId, response.SessionId)
	waitRuntimeRemoved(t, registry, response.SessionId)
	if err := os.RemoveAll(cwd); err != nil {
		t.Fatalf("RemoveAll(cwd) error = %v", err)
	}

	if _, err := registry.RerunSession(context.Background(), response.WorkspaceId, response.SessionId, &agent.RerunSessionReq{}); err == nil {
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
	err      error
}

func (m *fakeManager) Start(ctx context.Context, spec process.ProcessSpec) (termpty.Session, error) {
	m.specs = append(m.specs, spec)
	if m.err != nil {
		return nil, m.err
	}
	if len(m.sessions) > 0 {
		session := m.sessions[0]
		m.sessions = m.sessions[1:]
		return session, nil
	}
	return m.session, nil
}

type fakeSession struct {
	output       chan []byte
	done         chan termpty.Result
	closeSignal  chan struct{}
	writeMu      sync.Mutex
	written      []byte
	writeStarted chan struct{}
	writeRelease chan struct{}
	resizes      []process.TerminalSize
	resizeErrs   []error
	closed       bool
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
	if s.writeStarted != nil {
		select {
		case <-s.writeStarted:
		default:
			close(s.writeStarted)
		}
	}
	if s.writeRelease != nil {
		<-s.writeRelease
	}
	s.writeMu.Lock()
	s.written = append(s.written, p...)
	s.writeMu.Unlock()
	return len(p), nil
}

func (s *fakeSession) writtenCopy() []byte {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	out := make([]byte, len(s.written))
	copy(out, s.written)
	return out
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
