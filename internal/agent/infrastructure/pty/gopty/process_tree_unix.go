//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package gopty

import (
	"errors"
	"os"
	"syscall"

	gopty "github.com/aymanbagabas/go-pty"
)

func attachProcessTree(cmd *gopty.Cmd) (func() error, func() error, error) {
	if cmd == nil || cmd.Process == nil {
		return nil, nil, nil
	}
	pid := cmd.Process.Pid
	return func() error {
		if pid <= 0 {
			return nil
		}
		err := syscall.Kill(-pid, syscall.SIGKILL)
		if err == nil || errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if cmd.Process != nil {
			if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				return errors.Join(err, killErr)
			}
		}
		return err
	}, nil, nil
}
