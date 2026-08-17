package api

import (
	"fmt"
	"net/http"
	"testing"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
	sessionmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/session"
	shortcutmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/shortcut"
	workspacemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	tunnel "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

func TestFileErrorDetailsExposeCurrentRevisionWithoutWorkspacePath(t *testing.T) {
	details := fileErrorDetails(filemodel.Conflict(&filemodel.Entry{Path: "nested/notes.txt", Name: "notes.txt", Kind: filemodel.EntryKindFile, Size: 12, Revision: "current-revision"}))
	if details.GetFields()["current_entry"].GetStructValue().GetFields()["revision"].GetStringValue() != "current-revision" {
		t.Fatalf("conflict details = %v, want current revision", details)
	}
	if details.GetFields()["current_entry"].GetStructValue().GetFields()["path"].GetStringValue() != "nested/notes.txt" {
		t.Fatalf("conflict details = %v, want logical path", details)
	}
}

func TestRuntimeErrorResponsePreservesFileBusinessStatus(t *testing.T) {
	status, code, message := runtimeErrorResponse(filemodel.Conflict(&filemodel.Entry{Path: "notes.txt"}))
	if status != http.StatusConflict || code != "revision_conflict" {
		t.Fatalf("response = (%d, %q, %q), want 409 revision_conflict", status, code, message)
	}
}

func TestRuntimeErrorResponsePreservesWorkspaceRootUnavailable(t *testing.T) {
	status, code, _ := runtimeErrorResponse(filemodel.NewError("workspace_root_unavailable", "The workspace root is unavailable."))
	if status != http.StatusUnprocessableEntity || code != "workspace_root_unavailable" {
		t.Fatalf("response = (%d, %q), want 422 workspace_root_unavailable", status, code)
	}
}

func TestRuntimeErrorResponsePreservesRemoteFileBusinessStatus(t *testing.T) {
	err := &tunnel.RemoteError{Response: &shared.ErrorResp{Code: "file_not_found", Error: "The workspace entry was not found."}}
	status, code, _ := runtimeErrorResponse(err)
	if status != http.StatusNotFound || code != "file_not_found" {
		t.Fatalf("response = (%d, %q), want 404 file_not_found", status, code)
	}
}

func TestRuntimeErrorResponsePreservesSessionNotAttachable(t *testing.T) {
	status, code, message := runtimeErrorResponse(sessionmodel.NotAttachable())
	if status != http.StatusNotFound || code != sessionmodel.CodeNotAttachable || message != "Session is not attachable." {
		t.Fatalf("response = (%d, %q, %q)", status, code, message)
	}
}

func TestRuntimeErrorResponsePreservesSessionClosed(t *testing.T) {
	status, code, message := runtimeErrorResponse(sessionmodel.Closed())
	if status != http.StatusConflict || code != sessionmodel.CodeClosed || message != "Session is closed." {
		t.Fatalf("response = (%d, %q, %q)", status, code, message)
	}
}

func TestRuntimeErrorResponsePreservesGitInvalidOperation(t *testing.T) {
	status, code, message := runtimeErrorResponse(gitmodel.InvalidOperation())
	if status != http.StatusBadRequest || code != gitmodel.CodeInvalidOperation || message != "The Git operation is invalid." {
		t.Fatalf("response = (%d, %q, %q)", status, code, message)
	}
}

func TestRuntimeErrorResponseMapsAppUsageAndConfig(t *testing.T) {
	status, code, message := runtimeErrorResponse(apperrors.Usage("cannot delete running session"))
	if status != http.StatusBadRequest || code != errorCodeBadRequest || message != "cannot delete running session" {
		t.Fatalf("usage response = (%d, %q, %q)", status, code, message)
	}
	status, code, message = runtimeErrorResponse(apperrors.Config("invalid session cwd", fmt.Errorf("/secret")))
	if status != http.StatusBadRequest || code != errorCodeBadRequest || message != errorMessageBadRequest {
		t.Fatalf("config response = (%d, %q, %q)", status, code, message)
	}
}

func TestRuntimeErrorResponsePreservesSessionP1Catalog(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "missing name", err: sessionmodel.MissingName(), status: http.StatusBadRequest, code: sessionmodel.CodeMissingName, message: "Session name is required."},
		{name: "missing command", err: sessionmodel.MissingCommand(), status: http.StatusBadRequest, code: sessionmodel.CodeMissingCommand, message: "Session command is required."},
		{name: "not found", err: sessionmodel.NotFound(), status: http.StatusNotFound, code: sessionmodel.CodeNotFound, message: "Session was not found."},
		{name: "workspace not found", err: sessionmodel.WorkspaceNotFound(), status: http.StatusNotFound, code: sessionmodel.CodeWorkspaceNotFound, message: "Workspace was not found."},
		{name: "not editable", err: sessionmodel.NotEditable(), status: http.StatusConflict, code: sessionmodel.CodeNotEditable, message: "Only stopped or failed sessions can change the launch command."},
		{name: "already running", err: sessionmodel.AlreadyRunning(), status: http.StatusConflict, code: sessionmodel.CodeAlreadyRunning, message: "Session is already running."},
		{name: "cannot rerun", err: sessionmodel.CannotRerunRunning(), status: http.StatusConflict, code: sessionmodel.CodeCannotRerunRunning, message: "A running session cannot be rerun."},
		{name: "cannot delete", err: sessionmodel.CannotDeleteRunning(), status: http.StatusConflict, code: sessionmodel.CodeCannotDeleteRunning, message: "A running session cannot be deleted."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, code, message := runtimeErrorResponse(tc.err)
			if status != tc.status || code != tc.code || message != tc.message {
				t.Fatalf("response = (%d, %q, %q), want (%d, %q, %q)", status, code, message, tc.status, tc.code, tc.message)
			}
		})
	}
}

func TestRuntimeErrorResponsePreservesWorkspaceAndShortcutCatalog(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "workspace running sessions", err: workspacemodel.CannotDeleteWithRunningSessions(), status: http.StatusConflict, code: workspacemodel.CodeCannotDeleteWithRunningSessions, message: "A workspace with running sessions cannot be deleted."},
		{name: "shortcut id required", err: shortcutmodel.IDRequired(), status: http.StatusBadRequest, code: shortcutmodel.CodeIDRequired, message: "Shortcut id is required."},
		{name: "shortcut not found", err: shortcutmodel.NotFound(), status: http.StatusNotFound, code: shortcutmodel.CodeNotFound, message: "Shortcut was not found."},
		{name: "shortcut disabled", err: shortcutmodel.Disabled(), status: http.StatusConflict, code: shortcutmodel.CodeDisabled, message: "Shortcut is disabled."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, code, message := runtimeErrorResponse(tc.err)
			if status != tc.status || code != tc.code || message != tc.message {
				t.Fatalf("response = (%d, %q, %q), want (%d, %q, %q)", status, code, message, tc.status, tc.code, tc.message)
			}
		})
	}
}

func TestRuntimeErrorResponsePreservesSessionP2InputCatalog(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "command shape", err: sessionmodel.InvalidCommandShape(), status: http.StatusBadRequest, code: sessionmodel.CodeInvalidCommandShape},
		{name: "command source", err: sessionmodel.InvalidCommandSource(), status: http.StatusBadRequest, code: sessionmodel.CodeInvalidCommandSource},
		{name: "source needs command", err: sessionmodel.CommandSourceRequiresCommand(), status: http.StatusBadRequest, code: sessionmodel.CodeCommandSourceNeedsCommand},
		{name: "update fields", err: sessionmodel.UpdateRequiresFields(), status: http.StatusBadRequest, code: sessionmodel.CodeUpdateRequiresFields},
		{name: "size", err: sessionmodel.InvalidSize(), status: http.StatusBadRequest, code: sessionmodel.CodeInvalidSize},
		{name: "cwd", err: sessionmodel.InvalidCwd(), status: http.StatusBadRequest, code: sessionmodel.CodeInvalidCwd},
		{name: "session ids", err: sessionmodel.SessionIDsRequired(), status: http.StatusBadRequest, code: sessionmodel.CodeSessionIDsRequired},
		{name: "duplicate session", err: sessionmodel.DuplicateSessionID(), status: http.StatusBadRequest, code: sessionmodel.CodeDuplicateSessionID},
		{name: "workspace ids", err: workspacemodel.IDsRequired(), status: http.StatusBadRequest, code: workspacemodel.CodeWorkspaceIDsRequired},
		{name: "duplicate workspace", err: workspacemodel.DuplicateID(), status: http.StatusBadRequest, code: workspacemodel.CodeDuplicateWorkspaceID},
		{name: "shortcut name", err: shortcutmodel.NameRequired(), status: http.StatusBadRequest, code: shortcutmodel.CodeNameRequired},
		{name: "shortcut command", err: shortcutmodel.CommandRequired(), status: http.StatusBadRequest, code: shortcutmodel.CodeCommandRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, code, message := runtimeErrorResponse(tc.err)
			if status != tc.status || code != tc.code || message == "" {
				t.Fatalf("response = (%d, %q, %q)", status, code, message)
			}
		})
	}
}
