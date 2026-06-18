package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	apperrors "termbridge-go/internal/errors"
	"termbridge-go/internal/process"
	termpty "termbridge-go/internal/pty"
)

const defaultInterruptGrace = 1500 * time.Millisecond

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type IO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Result struct {
	ExitCode int
	Exit     process.ExitResult
}

type CommandRunner struct {
	Manager        termpty.Manager
	Logger         Logger
	InterruptGrace time.Duration
}

func (r CommandRunner) Run(ctx context.Context, spec process.ProcessSpec, streams IO) (Result, error) {
	if r.Manager == nil {
		return Result{}, apperrors.Internal("command runner missing PTY manager", nil)
	}
	if streams.Stdout == nil {
		streams.Stdout = io.Discard
	}
	if streams.Stderr == nil {
		streams.Stderr = io.Discard
	}
	if streams.Stdin == nil {
		streams.Stdin = emptyReader{}
	}
	if r.InterruptGrace <= 0 {
		r.InterruptGrace = defaultInterruptGrace
	}
	if err := spec.Validate(); err != nil {
		return Result{}, apperrors.Runtime("invalid process spec", err)
	}

	resolved, err := process.ResolveExecutable(spec.Command)
	if err != nil {
		return Result{}, apperrors.Runtime("resolve executable", err)
	}
	spec = spec.WithResolvedCommand(resolved)

	terminal, err := prepareTerminal(streams.Stdin, streams.Stdout)
	if err != nil {
		return Result{}, apperrors.Runtime("prepare terminal", err)
	}
	defer func() {
		if err := terminal.Restore(); err != nil && r.Logger != nil {
			r.Logger.Warn("restore terminal", "error", err)
		}
	}()
	if terminal.Size.IsValid() {
		spec.InitialSize = terminal.Size
	}

	session, err := r.Manager.Start(ctx, spec)
	if err != nil {
		return Result{}, apperrors.Runtime("start command", err)
	}

	outputDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(streams.Stdout, session)
		outputDone <- err
	}()

	ctrlC := make(chan struct{}, 4)
	go relayInput(ctx, streams.Stdin, session, ctrlC)

	resizeCtx, stopResize := context.WithCancel(ctx)
	defer stopResize()
	go watchResize(resizeCtx, terminal.ResizeFD, spec.InitialSize, session, r.Logger)

	waitCh := make(chan termpty.Result, 1)
	go func() { waitCh <- session.Wait() }()

	signalCh := make(chan os.Signal, 2)
	signal.Notify(signalCh, os.Interrupt)
	defer signal.Stop(signalCh)

	mode := process.StopNone
	var escalation <-chan time.Time

	for {
		select {
		case result := <-waitCh:
			stopResize()
			_ = session.Close()
			r.waitOutput(outputDone)
			exit := process.InterpretExit(result.Err, result.ExitCode, mode)
			if result.Err != nil && !exit.Stopped && exit.Code == 0 {
				return Result{ExitCode: process.DefaultExitCode, Exit: exit}, apperrors.Runtime("wait command", result.Err)
			}
			return Result{ExitCode: exit.Code, Exit: exit}, nil
		case <-signalCh:
			mode, escalation = r.handleInterrupt(streams.Stderr, session, mode)
		case <-ctrlC:
			mode, escalation = r.handleInterrupt(streams.Stderr, session, mode)
		case <-escalation:
			mode, escalation = r.escalate(streams.Stderr, session, mode)
		case <-ctx.Done():
			mode = process.StopClose
			_, _ = fmt.Fprintln(streams.Stderr, "termbridge: context cancelled; closing PTY")
			_ = session.Close()
			escalation = time.After(r.InterruptGrace)
		}
	}
}

func (r CommandRunner) handleInterrupt(stderr io.Writer, session termpty.Session, mode process.StopMode) (process.StopMode, <-chan time.Time) {
	if mode == process.StopNone {
		_, _ = fmt.Fprintln(stderr, "termbridge: interrupting command; press Ctrl+C again to force stop")
		if err := session.Interrupt(); err != nil && r.Logger != nil {
			r.Logger.Warn("soft interrupt failed", "error", err)
		}
		return process.StopInterrupt, time.After(r.InterruptGrace)
	}
	_, _ = fmt.Fprintln(stderr, "termbridge: forcing command stop")
	if err := session.Close(); err != nil && r.Logger != nil {
		r.Logger.Warn("close PTY after interrupt", "error", err)
	}
	if err := session.KillTree(); err != nil && r.Logger != nil {
		r.Logger.Warn("kill process tree after interrupt", "error", err)
	}
	return process.StopKill, nil
}

func (r CommandRunner) escalate(stderr io.Writer, session termpty.Session, mode process.StopMode) (process.StopMode, <-chan time.Time) {
	switch mode {
	case process.StopInterrupt:
		_, _ = fmt.Fprintln(stderr, "termbridge: command did not stop; closing PTY")
		if err := session.Close(); err != nil && r.Logger != nil {
			r.Logger.Warn("close PTY after interrupt timeout", "error", err)
		}
		return process.StopClose, time.After(r.InterruptGrace)
	case process.StopClose:
		_, _ = fmt.Fprintln(stderr, "termbridge: command did not exit after close; killing process")
		if err := session.KillTree(); err != nil && r.Logger != nil {
			r.Logger.Warn("kill process tree after close timeout", "error", err)
		}
		return process.StopKill, nil
	default:
		return mode, nil
	}
}

func (r CommandRunner) waitOutput(done <-chan error) {
	select {
	case err := <-done:
		if err != nil && r.Logger != nil {
			r.Logger.Debug("PTY output relay ended", "error", err)
		}
	case <-time.After(500 * time.Millisecond):
		if r.Logger != nil {
			r.Logger.Debug("PTY output relay did not finish before timeout")
		}
	}
}

type emptyReader struct{}

func (emptyReader) Read([]byte) (int, error) { return 0, io.EOF }
