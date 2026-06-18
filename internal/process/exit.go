package process

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const (
	DefaultExitCode = 1
	ForcedExitCode  = 130
)

type StopMode int

const (
	StopNone StopMode = iota
	StopInterrupt
	StopClose
	StopKill
)

func (m StopMode) String() string {
	switch m {
	case StopInterrupt:
		return "interrupt"
	case StopClose:
		return "close"
	case StopKill:
		return "kill"
	default:
		return "none"
	}
}

type ExitResult struct {
	Code    int
	Stopped bool
	Forced  bool
	Closed  bool
	WaitErr error
}

// InterpretExit classifies the final process exit outcome for the runner.
func InterpretExit(waitErr error, code int, mode StopMode) ExitResult {
	result := ExitResult{Code: code, WaitErr: waitErr}

	if mode == StopClose || mode == StopKill {
		result.Stopped = true
		result.Forced = true
		result.Closed = true
		result.Code = ForcedExitCode
		return result
	}

	if mode == StopInterrupt {
		result.Stopped = true
	}

	if result.Code == 0 && waitErr != nil {
		if isWindowsClosedWaitError(waitErr) {
			result.Closed = true
			result.Code = ForcedExitCode
			return result
		}
		if isExitError(waitErr) {
			result.Code = DefaultExitCode
		}
	}

	if result.Code == 0 && waitErr != nil && result.Stopped {
		result.Code = ForcedExitCode
	}

	return result
}

func isExitError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

func isWindowsClosedWaitError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "0xc000013a")
}

func FormatExitHint(code int, mode StopMode) string {
	if code == ForcedExitCode && mode != StopNone {
		return fmt.Sprintf("stopped with exit code %d", code)
	}
	return fmt.Sprintf("exit code %d", code)
}
