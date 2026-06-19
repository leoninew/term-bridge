package webserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"termbridge-go/internal/config"
	"termbridge-go/internal/state"
	"termbridge-go/internal/webterminal"
)

func TestHealthRoute(t *testing.T) {
	server := New(Config{}, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(response, request)

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

	server.server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body []webterminal.SessionSummary
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

	server.server.Handler.ServeHTTP(response, request)

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

	server.server.Handler.ServeHTTP(response, request)

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

	server.server.Handler.ServeHTTP(response, request)

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

	server.server.Handler.ServeHTTP(response, request)

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

	server.server.Handler.ServeHTTP(response, request)

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

	server.server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body []webterminal.WorkspaceSummary
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

	server.server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body []webterminal.WorkspaceTreeNode
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

	server.server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestServerLogsRequestsWithConfiguredLimits(t *testing.T) {
	var logBuffer bytes.Buffer
	config := Config{Logger: slog.New(slog.NewJSONHandler(&logBuffer, nil))}
	config.RequestBodyLimit = 10
	config.ResponseBodyLimit = 12
	server := New(config, newTestRegistry(t))
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("User-Agent", "test-agent")
	response := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(response, request)

	entry := decodeLogEntry(t, logBuffer.String())
	assertLogValue(t, entry, "method", http.MethodGet)
	assertLogValue(t, entry, "path", "/api/health")
	assertLogNumber(t, entry, "status", http.StatusOK)
	responseBody, ok := entry["response_body"].(string)
	if !ok {
		t.Fatalf("expected response_body: %+v", entry)
	}
	if !strings.HasSuffix(responseBody, truncatedLogBodySuffix) {
		t.Fatalf("expected truncated response_body, got %q", responseBody)
	}
}

func newTestRegistry(t *testing.T) *webterminal.Registry {
	t.Helper()
	cwd := t.TempDir()
	return webterminal.NewRegistry(webterminal.Config{Cwd: cwd, Store: state.NewStore(t.TempDir()), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}})
}

func decodeAPIError(t *testing.T, response *httptest.ResponseRecorder) apiErrorResponse {
	t.Helper()
	var body apiErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}
