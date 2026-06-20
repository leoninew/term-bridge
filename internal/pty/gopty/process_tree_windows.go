//go:build windows

package gopty

import (
	"errors"
	"os"
	"unsafe"

	gopty "github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/windows"
)

func attachProcessTree(cmd *gopty.Cmd) (func() error, func() error, error) {
	if cmd == nil || cmd.Process == nil {
		return nil, nil, nil
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fallbackKillTree(cmd), nil, nil
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return fallbackKillTree(cmd), nil, nil
	}
	processHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return fallbackKillTree(cmd), nil, nil
	}
	defer windows.CloseHandle(processHandle)
	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		_ = windows.CloseHandle(job)
		return fallbackKillTree(cmd), nil, nil
	}
	closed := false
	cleanup := func() error {
		if closed {
			return nil
		}
		closed = true
		return windows.CloseHandle(job)
	}
	return cleanup, cleanup, nil
}

func fallbackKillTree(cmd *gopty.Cmd) func() error {
	return func() error {
		if cmd == nil || cmd.Process == nil {
			return nil
		}
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return nil
	}
}
