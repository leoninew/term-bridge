package file

import (
	"errors"
	"fmt"
)

type Error struct {
	Code    string
	Message string
	Entry   *Entry
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *Error) RuntimeErrorCode() string {
	if e == nil {
		return "internal_error"
	}
	return e.Code
}

func InvalidPath(message string) error {
	return &Error{Code: "invalid_file_path", Message: message}
}

func NewError(code string, message string) error {
	return &Error{Code: code, Message: message}
}

func Conflict(entry *Entry) error {
	return &Error{Code: "revision_conflict", Message: "The workspace entry changed.", Entry: entry}
}

func CodeOf(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return "internal_error"
}

func Errorf(code string, format string, args ...any) error {
	return NewError(code, fmt.Sprintf(format, args...))
}
