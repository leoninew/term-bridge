package gopty

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	gopty "github.com/aymanbagabas/go-pty"

	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
)

func TestManagerStartReturnsErrorWhenInitialResizeFails(t *testing.T) {
	resizeErr := errors.New("initial resize failed")
	pt := &initialResizeFakePTY{resizeErr: resizeErr}
	manager := Manager{newPTY: func() (gopty.Pty, error) { return pt, nil }}
	spec := process.ProcessSpec{CommandText: "fake-command", Cwd: t.TempDir(), InitialSize: process.TerminalSize{Cols: 123, Rows: 45}}

	session, err := manager.Start(context.Background(), spec)
	if err == nil {
		t.Fatal("Start() error = nil, want initial resize failure")
	}
	if session != nil {
		t.Fatalf("Start() session = %#v, want nil", session)
	}
	if !strings.Contains(err.Error(), "resize initial pty to 123x45") || !errors.Is(err, resizeErr) {
		t.Fatalf("Start() error = %v, want wrapped initial resize failure", err)
	}
	if pt.resizeCols != 123 || pt.resizeRows != 45 {
		t.Fatalf("Resize() size = %dx%d, want 123x45", pt.resizeCols, pt.resizeRows)
	}
	if !pt.closed {
		t.Fatal("Start() did not close PTY after initial resize failure")
	}
}

type initialResizeFakePTY struct {
	resizeErr  error
	resizeCols int
	resizeRows int
	closed     bool
}

func (p *initialResizeFakePTY) Read([]byte) (int, error)       { return 0, io.EOF }
func (p *initialResizeFakePTY) Write(data []byte) (int, error) { return len(data), nil }
func (p *initialResizeFakePTY) Close() error {
	p.closed = true
	return nil
}
func (p *initialResizeFakePTY) Name() string { return "fake-pty" }
func (p *initialResizeFakePTY) Command(string, ...string) *gopty.Cmd {
	return &gopty.Cmd{}
}
func (p *initialResizeFakePTY) CommandContext(context.Context, string, ...string) *gopty.Cmd {
	return &gopty.Cmd{}
}
func (p *initialResizeFakePTY) Resize(cols int, rows int) error {
	p.resizeCols = cols
	p.resizeRows = rows
	return p.resizeErr
}
func (p *initialResizeFakePTY) Fd() uintptr { return 0 }
