package runner

import (
	"context"
	"io"
	"time"

	"golang.org/x/term"

	termpty "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/pty"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
)

type terminalSession struct {
	Size     process.TerminalSize
	ResizeFD int
	restore  func() error
}

func (s terminalSession) Restore() error {
	if s.restore == nil {
		return nil
	}
	return s.restore()
}

func prepareTerminal(stdin io.Reader, stdout io.Writer) (terminalSession, error) {
	result := terminalSession{Size: process.DefaultTerminalSize(), ResizeFD: -1}

	if fd, ok := fileDescriptor(stdout); ok && term.IsTerminal(fd) {
		result.ResizeFD = fd
		if cols, rows, err := term.GetSize(fd); err == nil {
			result.Size = process.TerminalSize{Cols: cols, Rows: rows}.OrDefault()
		}
	}
	if result.ResizeFD == -1 {
		if fd, ok := fileDescriptor(stdin); ok && term.IsTerminal(fd) {
			result.ResizeFD = fd
			if cols, rows, err := term.GetSize(fd); err == nil {
				result.Size = process.TerminalSize{Cols: cols, Rows: rows}.OrDefault()
			}
		}
	}

	if fd, ok := fileDescriptor(stdin); ok && term.IsTerminal(fd) {
		state, err := term.MakeRaw(fd)
		if err != nil {
			return terminalSession{}, err
		}
		result.restore = func() error { return term.Restore(fd, state) }
	}

	return result, nil
}

func watchResize(ctx context.Context, fd int, initial process.TerminalSize, session termpty.Session, logger Logger) {
	if fd < 0 {
		return
	}
	last := initial.OrDefault()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cols, rows, err := term.GetSize(fd)
			if err != nil {
				continue
			}
			next := process.TerminalSize{Cols: cols, Rows: rows}.OrDefault()
			last = resizeIfChanged(next, last, session, logger)
		}
	}
}

func resizeIfChanged(next process.TerminalSize, last process.TerminalSize, session termpty.Session, logger Logger) process.TerminalSize {
	if next == last {
		return last
	}
	if err := session.Resize(next); err != nil {
		logger.Warn("resize PTY", "cols", next.Cols, "rows", next.Rows, "error_kind", apperrors.KindOf(err))
		return last
	}
	return next
}

type fdProvider interface {
	Fd() uintptr
}

func fileDescriptor(value any) (int, bool) {
	provider, ok := value.(fdProvider)
	if !ok {
		return 0, false
	}
	return int(provider.Fd()), true
}
