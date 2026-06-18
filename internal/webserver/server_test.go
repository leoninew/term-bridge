package webserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func newTestRegistry(t *testing.T) *webterminal.Registry {
	t.Helper()
	cwd := t.TempDir()
	return webterminal.NewRegistry(webterminal.Config{Cwd: cwd, Store: state.NewStore(t.TempDir()), LogDir: filepath.Join(cwd, "logs"), History: config.HistoryConfig{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 256}})
}
