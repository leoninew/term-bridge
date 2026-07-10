package gopty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	gopty "github.com/aymanbagabas/go-pty"

	termpty "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/pty"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
)

type Manager struct {
	newPTY func() (gopty.Pty, error)
}

func NewManager() Manager {
	return Manager{newPTY: gopty.New}
}

func (m Manager) Start(ctx context.Context, spec process.ProcessSpec) (termpty.Session, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	command, err := parseCommandText(spec.CommandText)
	if err != nil {
		return nil, err
	}

	newPTY := m.newPTY
	if newPTY == nil {
		newPTY = gopty.New
	}
	pt, err := newPTY()
	if err != nil {
		return nil, err
	}
	if size := spec.InitialSize.OrDefault(); size.IsValid() {
		if err := pt.Resize(size.Cols, size.Rows); err != nil {
			_ = pt.Close()
			return nil, fmt.Errorf("resize initial pty to %dx%d: %w", size.Cols, size.Rows, err)
		}
	}

	executable, err := exec.LookPath(command.executable)
	if err != nil {
		_ = pt.Close()
		return nil, fmt.Errorf("resolve executable: %w", err)
	}

	// Use context.Background() to prevent the process from being killed
	// when the HTTP request context is cancelled.
	// The process lifecycle is managed by the session runtime, not the HTTP request.
	cmd := pt.CommandContext(context.Background(), executable, command.args...)
	cmd.Dir = spec.Cwd
	cmd.Env = spec.Env
	startedAt := time.Now().UTC()
	if err := cmd.Start(); err != nil {
		_ = pt.Close()
		return nil, err
	}
	killTree, cleanupTree, err := attachProcessTree(cmd)
	if err != nil {
		_ = pt.Close()
		_ = cmd.Process.Kill()
		return nil, err
	}

	s := &session{
		pty:         pt,
		cmd:         cmd,
		killTree:    killTree,
		cleanupTree: cleanupTree,
		process: process.Record{
			SchemaVersion: 1,
			Pid:           cmd.Process.Pid,
			OwnerPid:      os.Getpid(),
			Executable:    executable,
			CommandLine:   spec.CommandText,
			Cwd:           spec.Cwd,
			StartedAt:     startedAt,
		},
		done: make(chan termpty.Result, 1),
	}
	go s.wait()
	return s, nil
}

type session struct {
	pty         gopty.Pty
	cmd         *gopty.Cmd
	killTree    func() error
	cleanupTree func() error
	process     process.Record
	done        chan termpty.Result
	once        sync.Once
}

func (s *session) Read(p []byte) (int, error) {
	return s.pty.Read(p)
}

func (s *session) Write(p []byte) (int, error) {
	return s.pty.Write(p)
}

func (s *session) Resize(size process.TerminalSize) error {
	size = size.OrDefault()
	return s.pty.Resize(size.Cols, size.Rows)
}

func (s *session) Interrupt() error {
	_, err := s.pty.Write([]byte{3})
	return err
}

func (s *session) Close() error {
	var err error
	s.once.Do(func() { err = s.pty.Close() })
	return err
}

func (s *session) KillTree() error {
	if s.killTree != nil {
		return s.killTree()
	}
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

func (s *session) Wait() termpty.Result {
	return <-s.done
}

func (s *session) ProcessInfo() process.Record {
	return s.process
}

func (s *session) wait() {
	err := s.cmd.Wait()
	if s.cleanupTree != nil {
		_ = s.cleanupTree()
	}
	code := 0
	if s.cmd.ProcessState != nil {
		code = s.cmd.ProcessState.ExitCode()
	}
	if err != nil && code == 0 {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}
	}
	if errors.Is(err, io.EOF) {
		err = nil
	}
	s.done <- termpty.Result{ExitCode: code, Err: err}
}
