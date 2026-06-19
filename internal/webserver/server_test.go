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
	if response.Body.String() == "" {
		t.Fatal("empty response body")
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
