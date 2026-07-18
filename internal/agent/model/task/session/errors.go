package session

import "errors"

const (
	CodeNotAttachable             = "session_not_attachable"
	CodeClosed                    = "session_closed"
	CodeNotFound                  = "session_not_found"
	CodeWorkspaceNotFound         = "workspace_not_found"
	CodeMissingName               = "missing_session_name"
	CodeMissingCommand            = "missing_session_command"
	CodeNotEditable               = "session_not_editable"
	CodeAlreadyRunning            = "session_already_running"
	CodeCannotRerunRunning        = "cannot_rerun_running_session"
	CodeCannotDeleteRunning       = "cannot_delete_running_session"
	CodeInvalidCommandShape       = "invalid_session_command_shape"
	CodeInvalidCommandSource      = "invalid_session_command_source"
	CodeCommandSourceNeedsCommand = "session_command_source_requires_command"
	CodeUpdateRequiresFields      = "session_update_requires_fields"
	CodeInvalidSize               = "invalid_terminal_size"
	CodeInvalidCwd                = "invalid_session_cwd"
	CodeSessionIDsRequired        = "session_ids_required"
	CodeDuplicateSessionID        = "duplicate_session_id"
)

const (
	messageNotAttachable             = "Session is not attachable."
	messageClosed                    = "Session is closed."
	messageNotFound                  = "Session was not found."
	messageWorkspaceNotFound         = "Workspace was not found."
	messageMissingName               = "Session name is required."
	messageMissingCommand            = "Session command is required."
	messageNotEditable               = "Only stopped or failed sessions can be edited."
	messageAlreadyRunning            = "Session is already running."
	messageCannotRerunRunning        = "A running session cannot be rerun."
	messageCannotDeleteRunning       = "A running session cannot be deleted."
	messageInvalidCommandShape       = "Session command must contain one raw command text."
	messageInvalidCommandSource      = "Session command source is invalid."
	messageCommandSourceNeedsCommand = "Session command source requires a command."
	messageUpdateRequiresFields      = "Session update requires a name or command."
	messageInvalidSize               = "Terminal size is invalid."
	messageInvalidCwd                = "Session working directory is invalid."
	messageSessionIDsRequired        = "Session ids are required."
	messageDuplicateSessionID        = "Session ids contain duplicates."
)

// Error is a session-domain error with a stable code and client-safe message.
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

func NotAttachable() error { return &Error{Code: CodeNotAttachable, Message: messageNotAttachable} }
func Closed() error        { return &Error{Code: CodeClosed, Message: messageClosed} }
func NotFound() error      { return &Error{Code: CodeNotFound, Message: messageNotFound} }
func WorkspaceNotFound() error {
	return &Error{Code: CodeWorkspaceNotFound, Message: messageWorkspaceNotFound}
}
func MissingName() error    { return &Error{Code: CodeMissingName, Message: messageMissingName} }
func MissingCommand() error { return &Error{Code: CodeMissingCommand, Message: messageMissingCommand} }
func NotEditable() error    { return &Error{Code: CodeNotEditable, Message: messageNotEditable} }
func AlreadyRunning() error { return &Error{Code: CodeAlreadyRunning, Message: messageAlreadyRunning} }
func CannotRerunRunning() error {
	return &Error{Code: CodeCannotRerunRunning, Message: messageCannotRerunRunning}
}
func CannotDeleteRunning() error {
	return &Error{Code: CodeCannotDeleteRunning, Message: messageCannotDeleteRunning}
}
func InvalidCommandShape() error {
	return &Error{Code: CodeInvalidCommandShape, Message: messageInvalidCommandShape}
}
func InvalidCommandSource() error {
	return &Error{Code: CodeInvalidCommandSource, Message: messageInvalidCommandSource}
}
func CommandSourceRequiresCommand() error {
	return &Error{Code: CodeCommandSourceNeedsCommand, Message: messageCommandSourceNeedsCommand}
}
func UpdateRequiresFields() error {
	return &Error{Code: CodeUpdateRequiresFields, Message: messageUpdateRequiresFields}
}
func InvalidSize() error { return &Error{Code: CodeInvalidSize, Message: messageInvalidSize} }
func InvalidCwd() error  { return &Error{Code: CodeInvalidCwd, Message: messageInvalidCwd} }
func SessionIDsRequired() error {
	return &Error{Code: CodeSessionIDsRequired, Message: messageSessionIDsRequired}
}
func DuplicateSessionID() error {
	return &Error{Code: CodeDuplicateSessionID, Message: messageDuplicateSessionID}
}

func CodeOf(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}
