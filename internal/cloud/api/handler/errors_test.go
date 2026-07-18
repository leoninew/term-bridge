package api

import (
	"net/http"
	"testing"

	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	tunnel "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

func TestDomainErrorResponsePreservesRemoteConflict(t *testing.T) {
	status, code, _ := domainErrorResponse("revision_conflict", "The workspace entry changed.")
	if status != http.StatusConflict || code != "revision_conflict" {
		t.Fatalf("response = (%d, %q), want 409 revision_conflict", status, code)
	}
}

func TestDomainErrorResponsePreservesWorkspaceRootUnavailable(t *testing.T) {
	status, code, _ := domainErrorResponse("workspace_root_unavailable", "The workspace root is unavailable.")
	if status != http.StatusUnprocessableEntity || code != "workspace_root_unavailable" {
		t.Fatalf("response = (%d, %q), want 422 workspace_root_unavailable", status, code)
	}
}

func TestDomainErrorResponsePreservesRemoteNotFound(t *testing.T) {
	remote := &tunnel.RemoteError{Response: &shared.ErrorResp{Code: "workspace_not_found", Error: "Workspace was not found."}}
	status, code, _ := domainErrorResponse(remote.Response.GetCode(), remote.Response.GetError())
	if status != http.StatusNotFound || code != "workspace_not_found" {
		t.Fatalf("response = (%d, %q), want 404 workspace_not_found", status, code)
	}
}
