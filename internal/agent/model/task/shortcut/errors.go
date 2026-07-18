package shortcut

import "errors"

const (
	CodeIDRequired      = "shortcut_id_required"
	CodeNotFound        = "shortcut_not_found"
	CodeDisabled        = "shortcut_disabled"
	CodeNameRequired    = "shortcut_name_required"
	CodeCommandRequired = "shortcut_command_required"
)

const (
	messageIDRequired      = "Shortcut id is required."
	messageNotFound        = "Shortcut was not found."
	messageDisabled        = "Shortcut is disabled."
	messageNameRequired    = "Shortcut name is required."
	messageCommandRequired = "Shortcut command is required."
)

// Error is a shortcut-domain error with a stable code and client-safe message.
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

func IDRequired() error   { return &Error{Code: CodeIDRequired, Message: messageIDRequired} }
func NotFound() error     { return &Error{Code: CodeNotFound, Message: messageNotFound} }
func Disabled() error     { return &Error{Code: CodeDisabled, Message: messageDisabled} }
func NameRequired() error { return &Error{Code: CodeNameRequired, Message: messageNameRequired} }
func CommandRequired() error {
	return &Error{Code: CodeCommandRequired, Message: messageCommandRequired}
}

func CodeOf(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}
