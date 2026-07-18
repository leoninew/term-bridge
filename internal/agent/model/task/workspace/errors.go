package workspace

import "errors"

const (
	CodeCannotDeleteWithRunningSessions = "cannot_delete_workspace_with_running_sessions"
	CodeWorkspaceIDsRequired            = "workspace_ids_required"
	CodeDuplicateWorkspaceID            = "duplicate_workspace_id"
)

const (
	messageCannotDeleteWithRunningSessions = "A workspace with running sessions cannot be deleted."
	messageWorkspaceIDsRequired            = "Workspace ids are required."
	messageDuplicateWorkspaceID            = "Workspace ids contain duplicates."
)

// Error is a workspace-domain error with a stable code and client-safe message.
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

func CannotDeleteWithRunningSessions() error {
	return &Error{Code: CodeCannotDeleteWithRunningSessions, Message: messageCannotDeleteWithRunningSessions}
}

func IDsRequired() error {
	return &Error{Code: CodeWorkspaceIDsRequired, Message: messageWorkspaceIDsRequired}
}

func DuplicateID() error {
	return &Error{Code: CodeDuplicateWorkspaceID, Message: messageDuplicateWorkspaceID}
}

func CodeOf(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}
