package api

import (
	"net/http"
	"testing"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
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
