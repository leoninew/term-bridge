package pty

import (
	"context"
	"io"

	"termbridge-go/internal/process"
)

type Manager interface {
	Start(ctx context.Context, spec process.ProcessSpec) (Session, error)
}

type Session interface {
	io.Reader
	io.Writer
	Resize(size process.TerminalSize) error
	Interrupt() error
	Close() error
	KillTree() error
	Wait() Result
}

type Result struct {
	ExitCode int
	Err      error
}
