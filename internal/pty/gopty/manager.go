package gopty

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	gopty "github.com/aymanbagabas/go-pty"

	"termbridge-go/internal/process"
	termpty "termbridge-go/internal/pty"
)

type Manager struct{}

func NewManager() Manager {
	return Manager{}
}

func (m Manager) Start(ctx context.Context, spec process.ProcessSpec) (termpty.Session, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}

	pt, err := gopty.New()
	if err != nil {
		return nil, err
	}
	if size := spec.InitialSize.OrDefault(); size.IsValid() {
		_ = pt.Resize(size.Cols, size.Rows)
	}

	cmd := pt.CommandContext(ctx, spec.EffectiveCommand(), spec.Args...)
	cmd.Dir = spec.Cwd
	cmd.Env = spec.Env
	startedAt := time.Now().UTC()
	if err := cmd.Start(); err != nil {
		_ = pt.Close()
		return nil, err
	}

	s := &session{
		pty: pt,
		cmd: cmd,
		process: process.Record{
			SchemaVersion: 1,
			PID:           cmd.Process.Pid,
			OwnerPID:      os.Getpid(),
			Executable:    spec.EffectiveCommand(),
			CommandLine:   strings.Join(append([]string{spec.Command}, spec.Args...), " "),
			Cwd:           spec.Cwd,
			StartedAt:     startedAt,
		},
		done: make(chan termpty.Result, 1),
	}
	go s.wait()
	return s, nil
}

type session struct {
	pty     gopty.Pty
	cmd     *gopty.Cmd
	process process.Record
	done    chan termpty.Result
	once    sync.Once
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
	code := 0
	if s.cmd.ProcessState != nil {
		code = s.cmd.ProcessState.ExitCode()
	}
	if err != nil && code == 0 {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ProcessState != nil {
			code = exitErr.ProcessState.ExitCode()
		}
	}
	if errors.Is(err, io.EOF) {
		err = nil
	}
	s.done <- termpty.Result{ExitCode: code, Err: err}
}
