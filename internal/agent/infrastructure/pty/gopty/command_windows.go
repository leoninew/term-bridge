//go:build windows

package gopty

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func parseCommandArguments(commandText string) ([]string, error) {
	commandLine, err := windows.UTF16PtrFromString(commandText)
	if err != nil {
		return nil, err
	}
	var argc int32
	argv, err := windows.CommandLineToArgv(commandLine, &argc)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(argv)))) }()

	values := unsafe.Slice((**uint16)(unsafe.Pointer(argv)), argc)
	args := make([]string, 0, argc)
	for _, value := range values {
		args = append(args, windows.UTF16PtrToString(value))
	}
	return args, nil
}
