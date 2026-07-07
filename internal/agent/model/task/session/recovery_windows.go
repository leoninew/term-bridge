//go:build windows

package session

import (
	"strings"
	"syscall"
	"unsafe"

	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"

	"golang.org/x/sys/windows"
)

const stillActive = 259

type OSAliveChecker struct{}

func (OSAliveChecker) Check(record process.Record) AliveStatus {
	if record.Pid <= 0 {
		return AliveMissing
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(record.Pid))
	if err != nil {
		return AliveMissing
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return AliveUnverified
	}
	if code != stillActive {
		return AliveMissing
	}
	if record.Executable == "" {
		return AliveMatched
	}
	if sameExecutable(handle, record.Executable) {
		return AliveMatched
	}
	return AliveUnverified
}

func sameExecutable(handle windows.Handle, executable string) bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	queryFullProcessImageName := kernel32.NewProc("QueryFullProcessImageNameW")
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	ret, _, _ := queryFullProcessImageName.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ret == 0 || size == 0 {
		return false
	}
	return strings.EqualFold(windows.UTF16ToString(buf[:size]), executable)
}
