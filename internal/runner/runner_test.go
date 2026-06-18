package runner

import (
	"bytes"
	"context"
	"io"
	"testing"

	"termbridge-go/internal/process"
	termpty "termbridge-go/internal/pty"
)

func TestRunReturnsUserExitCode(t *testing.T) {
	manager := &fakeManager{session: newFakePTYSession("hello", termpty.Result{ExitCode: 7})}
	r := CommandRunner{Manager: manager}

	result, err := r.Run(context.Background(), process.ProcessSpec{Command: "go", Cwd: t.TempDir(), InitialSize: process.DefaultTerminalSize()}, IO{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", result.ExitCode)
	}
}

type fakeManager struct {
	session termpty.Session
}

func (m *fakeManager) Start(context.Context, process.ProcessSpec) (termpty.Session, error) {
	return m.session, nil
}

type fakePTYSession struct {
	output *bytes.Reader
	input  bytes.Buffer
	result termpty.Result
}

func newFakePTYSession(output string, result termpty.Result) *fakePTYSession {
	return &fakePTYSession{output: bytes.NewReader([]byte(output)), result: result}
}

func (s *fakePTYSession) Read(p []byte) (int, error)        { return s.output.Read(p) }
func (s *fakePTYSession) Write(p []byte) (int, error)       { return s.input.Write(p) }
func (s *fakePTYSession) Resize(process.TerminalSize) error { return nil }
func (s *fakePTYSession) Interrupt() error                  { return nil }
func (s *fakePTYSession) Close() error                      { return nil }
func (s *fakePTYSession) KillTree() error                   { return nil }
func (s *fakePTYSession) Wait() termpty.Result              { return s.result }
