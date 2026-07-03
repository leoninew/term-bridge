//go:build !windows

package session

import (
	"os"
	"syscall"

	"termbridge-go/internal/agent/domain/process"
)

type OSAliveChecker struct{}

func (OSAliveChecker) Check(record process.Record) AliveStatus {
	if record.Pid <= 0 {
		return AliveMissing
	}
	proc, err := os.FindProcess(record.Pid)
	if err != nil {
		return AliveMissing
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return AliveMissing
	}
	return AliveMatched
}
