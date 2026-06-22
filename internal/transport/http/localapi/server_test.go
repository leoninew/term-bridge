package localapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	terminalapp "termbridge-go/internal/application/terminal"
	"termbridge-go/internal/infrastructure/config"
	"termbridge-go/internal/infrastructure/repository/state"
)

func TestHealthRoute(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body = %#v", body)
	}
}

func TestSessionsRouteListsEmptySessions(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body []terminalapp.SessionSummary
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body == nil {
		t.Fatalf("body = %#v, want array", body)
	}
}

func TestCreateSessionRequiresName(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(`{"command":["go","version"]}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	body := decodeAPIError(t, response)
	if body.Code != "usage_error" || body.Message != "missing session name" || body.Error != "" {
		t.Fatalf("body = %#v", body)
	}
}

func TestInvalidRequestIncludesDebugErrorWhenEnabled(t *testing.T) {
	server := New(Config{DebugErrors: true}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(`{"name"`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	body := decodeAPIError(t, response)
	if body.Code != "invalid_request" || body.Message != "invalid request" || body.Error == "" {
		t.Fatalf("body = %#v", body)
	}
}

func TestMissingSessionReturnsNotFoundError(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodGet, "/api/sessions/missing", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	body := decodeAPIError(t, response)
	if body.Code != "not_found" || body.Message != "session not found" || body.Error != "" {
		t.Fatalf("body = %#v", body)
	}
}

func TestRouteNotFoundReturnsAPIError(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodGet, "/api/sessions/missing/unknown", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	body := decodeAPIError(t, response)
	if body.Code != "not_found" || body.Message != "not found" || body.Error != "" {
		t.Fatalf("body = %#v", body)
	}
}

func TestMethodNotAllowedReturnsAPIError(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/tree", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.Code)
	}
	body := decodeAPIError(t, response)
	if body.Code != "method_not_allowed" || body.Message != "method not allowed" || body.Error != "" {
		t.Fatalf("body = %#v", body)
	}
}

func TestWorkspacesRouteListsRawArray(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodGet, "/api/workspaces", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body []terminalapp.WorkspaceSummary
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body == nil {
		t.Fatalf("body = %#v, want array", body)
	}
}

func TestWorkspaceTreeRoute(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodGet, "/api/workspaces/tree", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body []terminalapp.WorkspaceTreeNode
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body == nil {
		t.Fatalf("body = %#v, want array", body)
	}
}

func TestWorkspaceOrderRequiresIds(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodPatch, "/api/workspaces/order", strings.NewReader(`{"workspace_ids":[]}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func newTestRegistry(t *testing.T) *terminalapp.Registry {
	t.Helper()
	cwd := t.TempDir()
	return terminalapp.NewRegistry(terminalapp.Config{Cwd: cwd, Store: state.NewStore(t.TempDir()), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}})
}

func decodeAPIError(t *testing.T, response *httptest.ResponseRecorder) apiErrorResponse {
	t.Helper()
	var body apiErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}
