package git

import "errors"

const CodeInvalidOperation = "invalid_git_operation"

const messageInvalidOperation = "The Git operation is invalid."

// Error is a Git-domain error with a stable code and client-safe message.
type Error struct {
	Code    string
	Message string
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

func (e *Error) RuntimeErrorMessage() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func InvalidOperation() error {
	return &Error{Code: CodeInvalidOperation, Message: messageInvalidOperation}
}

func NewError(code string, message string) error {
	return &Error{Code: code, Message: message}
}

func CodeOf(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}
