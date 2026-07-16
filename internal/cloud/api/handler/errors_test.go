package api

import (
	"net/http"
	"testing"

	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	tunnel "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

func TestFileErrorResponsePreservesRemoteConflict(t *testing.T) {
	status, code, _ := fileErrorResponse("revision_conflict", "The workspace entry changed.")
	if status != http.StatusConflict || code != "revision_conflict" {
		t.Fatalf("response = (%d, %q), want 409 revision_conflict", status, code)
	}
}

func TestFileErrorResponsePreservesWorkspaceRootUnavailable(t *testing.T) {
	status, code, _ := fileErrorResponse("workspace_root_unavailable", "The workspace root is unavailable.")
	if status != http.StatusUnprocessableEntity || code != "workspace_root_unavailable" {
		t.Fatalf("response = (%d, %q), want 422 workspace_root_unavailable", status, code)
	}
}

func TestFileErrorResponsePreservesRemoteNotFound(t *testing.T) {
	remote := &tunnel.RemoteError{Response: &shared.ErrorResp{Code: "workspace_not_found", Error: "Workspace was not found."}}
	status, code, _ := fileErrorResponse(remote.Response.GetCode(), remote.Response.GetError())
	if status != http.StatusNotFound || code != "workspace_not_found" {
		t.Fatalf("response = (%d, %q), want 404 workspace_not_found", status, code)
	}
}
