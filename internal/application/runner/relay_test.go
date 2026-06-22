package runner

import (
	"bytes"
	"testing"

	"termbridge-go/internal/domain/process"
	termpty "termbridge-go/internal/infrastructure/pty"
)

func TestWriteInputChunkInterceptsCtrlC(t *testing.T) {
	session := &fakeRelaySession{}
	ctrlC := make(chan struct{}, 1)

	writeInputChunk(session, []byte("abc\x03def"), ctrlC)

	if got := session.input.String(); got != "abcdef" {
		t.Fatalf("input = %q", got)
	}
	select {
	case <-ctrlC:
	default:
		t.Fatal("ctrlC was not signaled")
	}
}

type fakeRelaySession struct {
	output bytes.Buffer
	input  bytes.Buffer
}

func (s *fakeRelaySession) Read(p []byte) (int, error)        { return s.output.Read(p) }
func (s *fakeRelaySession) Write(p []byte) (int, error)       { return s.input.Write(p) }
func (s *fakeRelaySession) Resize(process.TerminalSize) error { return nil }
func (s *fakeRelaySession) Interrupt() error                  { return nil }
func (s *fakeRelaySession) Close() error                      { return nil }
func (s *fakeRelaySession) KillTree() error                   { return nil }
func (s *fakeRelaySession) Wait() termpty.Result              { return termpty.Result{} }
