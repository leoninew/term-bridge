package errors

import (
	"errors"
	"fmt"
)

const (
	ExitSuccess = 0
	ExitGeneral = 1
	ExitUsage   = 2
	ExitConfig  = 3
)

type Kind string

const (
	KindUsage    Kind = "usage"
	KindConfig   Kind = "config"
	KindInternal Kind = "internal"
	KindRuntime  Kind = "runtime"
)

type Error struct {
	Kind Kind
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e.Msg == "" && e.Err != nil {
		return e.Err.Error()
	}
	if e.Err == nil {
		return e.Msg
	}
	return e.Msg + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func Usage(message string) error {
	return &Error{Kind: KindUsage, Msg: message}
}

func Config(message string, err error) error {
	return &Error{Kind: KindConfig, Msg: message, Err: err}
}

func Internal(message string, err error) error {
	return &Error{Kind: KindInternal, Msg: message, Err: err}
}

func Runtime(message string, err error) error {
	return &Error{Kind: KindRuntime, Msg: message, Err: err}
}

func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		switch appErr.Kind {
		case KindUsage:
			return ExitUsage
		case KindConfig:
			return ExitConfig
		default:
			return ExitGeneral
		}
	}
	return ExitGeneral
}

func KindOf(err error) Kind {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Kind
	}
	return KindInternal
}

func IsUsage(err error) bool {
	return KindOf(err) == KindUsage
}

func IsConfig(err error) bool {
	return KindOf(err) == KindConfig
}

func FormatUser(err error) string {
	if err == nil {
		return ""
	}
	var appErr *Error
	if errors.As(err, &appErr) && appErr.Msg != "" {
		if appErr.Err == nil {
			return appErr.Msg
		}
		return fmt.Sprintf("%s: %v", appErr.Msg, appErr.Err)
	}
	return err.Error()
}
