package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	termpty "termbridge-go/internal/agent/infrastructure/pty"
	"termbridge-go/internal/agent/model/task/process"
)

func TestRunReturnsUserExitCode(t *testing.T) {
	manager := &fakeManager{session: newFakePTYSession("hello", termpty.Result{ExitCode: 7})}
	r := CommandRunner{Manager: manager, Logger: slog.Default()}

	result, err := r.Run(context.Background(), process.ProcessSpec{Command: "go", Cwd: t.TempDir(), InitialSize: process.DefaultTerminalSize()}, IO{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", result.ExitCode)
	}
}

func TestResizeIfChangedRetriesAfterFailure(t *testing.T) {
	session := newFakePTYSession("", termpty.Result{ExitCode: 0})
	session.resizeErrs = []error{errors.New("resize failed")}
	last := process.TerminalSize{Cols: 80, Rows: 25}
	next := process.TerminalSize{Cols: 120, Rows: 32}

	last = resizeIfChanged(next, last, session, slog.Default())
	if last != (process.TerminalSize{Cols: 80, Rows: 25}) {
		t.Fatalf("last after failure = %#v, want original size", last)
	}
	last = resizeIfChanged(next, last, session, slog.Default())
	if last != next {
		t.Fatalf("last after retry = %#v, want %#v", last, next)
	}
	if len(session.resizes) != 2 || session.resizes[0] != next || session.resizes[1] != next {
		t.Fatalf("resizes = %#v, want two attempts to %#v", session.resizes, next)
	}
}

func TestHandleInterruptEscalatesToKillTree(t *testing.T) {
	session := newFakePTYSession("", termpty.Result{ExitCode: 0})
	var stderr bytes.Buffer
	var stops []string
	r := CommandRunner{InterruptGrace: time.Millisecond, Logger: slog.Default(), Hooks: Hooks{OnStopping: func(mode process.StopMode, reason string) {
		stops = append(stops, mode.String()+":"+reason)
	}}}

	mode, escalation := r.handleInterrupt(&stderr, session, process.StopNone)
	if mode != process.StopInterrupt || escalation == nil {
		t.Fatalf("first mode=%q escalation=%v", mode, escalation)
	}
	if session.interrupts != 1 || session.closes != 0 || session.kills != 0 {
		t.Fatalf("after interrupt: interrupts=%d closes=%d kills=%d", session.interrupts, session.closes, session.kills)
	}
	mode, escalation = r.escalate(&stderr, session, mode)
	if mode != process.StopClose || escalation == nil {
		t.Fatalf("close mode=%q escalation=%v", mode, escalation)
	}
	if session.closes != 1 || session.kills != 0 {
		t.Fatalf("after close escalation: closes=%d kills=%d", session.closes, session.kills)
	}
	mode, escalation = r.escalate(&stderr, session, mode)
	if mode != process.StopKill || escalation != nil {
		t.Fatalf("kill mode=%q escalation=%v", mode, escalation)
	}
	if session.kills != 1 {
		t.Fatalf("kills=%d, want 1", session.kills)
	}
	for _, want := range []string{"interrupt:interrupt_requested", "close:interrupt_timeout", "kill:close_timeout"} {
		if !containsString(stops, want) {
			t.Fatalf("stops missing %q: %#v", want, stops)
		}
	}
	for _, want := range []string{"interrupting command", "closing PTY", "killing process"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q: %s", want, stderr.String())
		}
	}
}

type fakeManager struct {
	session termpty.Session
}

func (m *fakeManager) Start(context.Context, process.ProcessSpec) (termpty.Session, error) {
	return m.session, nil
}

type fakePTYSession struct {
	output     *bytes.Reader
	input      bytes.Buffer
	result     termpty.Result
	resizes    []process.TerminalSize
	resizeErrs []error
	interrupts int
	closes     int
	kills      int
}

func newFakePTYSession(output string, result termpty.Result) *fakePTYSession {
	return &fakePTYSession{output: bytes.NewReader([]byte(output)), result: result}
}

func (s *fakePTYSession) Read(p []byte) (int, error)  { return s.output.Read(p) }
func (s *fakePTYSession) Write(p []byte) (int, error) { return s.input.Write(p) }
func (s *fakePTYSession) Resize(size process.TerminalSize) error {
	s.resizes = append(s.resizes, size)
	if len(s.resizeErrs) > 0 {
		err := s.resizeErrs[0]
		s.resizeErrs = s.resizeErrs[1:]
		return err
	}
	return nil
}
func (s *fakePTYSession) Interrupt() error     { s.interrupts++; return nil }
func (s *fakePTYSession) Close() error         { s.closes++; return nil }
func (s *fakePTYSession) KillTree() error      { s.kills++; return nil }
func (s *fakePTYSession) Wait() termpty.Result { return s.result }

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
