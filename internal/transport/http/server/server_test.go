package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerLogsUnifiedBackendRequests(t *testing.T) {
	var logBuffer bytes.Buffer
	localHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"local"}`))
	})
	gatewayHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/gateway/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"gateway"}`))
	})
	server := New(Config{Logger: slog.New(slog.NewJSONHandler(&logBuffer, nil)), RequestBodyLimit: 4096, ResponseBodyLimit: 4096}, localHandler, gatewayHandler)

	request := httptest.NewRequest(http.MethodGet, "/api/gateway/health?x=1", nil)
	request.Header.Set("User-Agent", "test-agent")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	entries := decodeLogEntries(t, logBuffer.String())
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d: %+v", len(entries), entries)
	}
	started := entries[0]
	completed := entries[1]
	assertLogValue(t, started, "msg", "request started")
	assertLogValue(t, completed, "msg", "request completed")
	assertLogValue(t, started, "method", http.MethodGet)
	assertLogValue(t, started, "path", "/api/gateway/health")
	assertLogValue(t, started, "uri", "/api/gateway/health?x=1")
	assertLogNumber(t, completed, "status", http.StatusOK)
	assertLogValue(t, completed, "response_body", response.Body.String())
	if started["request_id"] == "" || started["request_id"] != completed["request_id"] {
		t.Fatalf("expected matching request_id, got started=%#v completed=%#v", started["request_id"], completed["request_id"])
	}
}

func decodeLogEntries(t *testing.T, content string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("expected log entry")
	}
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode log entry: %v\n%s", err, content)
		}
		entries = append(entries, entry)
	}
	return entries
}

func assertLogValue(t *testing.T, entry map[string]any, key string, want string) {
	t.Helper()
	if got, ok := entry[key].(string); !ok || got != want {
		t.Fatalf("expected %s=%q, got %#v", key, want, entry[key])
	}
}

func assertLogNumber(t *testing.T, entry map[string]any, key string, want int) {
	t.Helper()
	got, ok := entry[key].(float64)
	if !ok || int(got) != want {
		t.Fatalf("expected %s=%d, got %#v", key, want, entry[key])
	}
}
