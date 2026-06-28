package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerServesStaticFilesWithSPAFallbackAndKeepsAPIRoutes(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html>app</html>"), 0o644); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}
	assetsDir := filepath.Join(staticDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(assets) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte("console.log('app')"), 0o644); err != nil {
		t.Fatalf("WriteFile(asset) error = %v", err)
	}
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"gateway"}`))
		default:
			http.NotFound(w, r)
		}
	})
	server := New(Config{StaticDir: staticDir, Logger: slog.Default()}, apiHandler)

	cases := []struct {
		path string
		want string
	}{
		{path: "/", want: "<html>app</html>"},
		{path: "/sessions", want: "<html>app</html>"},
		{path: "/settings", want: "<html>app</html>"},
		{path: "/assets/app.js", want: "console.log('app')"},
		{path: "/api/health", want: `{"status":"gateway"}`},
	}
	for _, tc := range cases {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), tc.want) {
			t.Fatalf("GET %s = %d %q, want body containing %q", tc.path, response.Code, response.Body.String(), tc.want)
		}
	}

	apiResponse := httptest.NewRecorder()
	server.ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	if apiResponse.Code != http.StatusNotFound || strings.Contains(apiResponse.Body.String(), "<html>app</html>") {
		t.Fatalf("/api/missing = %d %q, want API 404 without SPA fallback", apiResponse.Code, apiResponse.Body.String())
	}
}

func TestServerListenRejectsInvalidStaticDir(t *testing.T) {
	server := New(Config{ServerUrl: "http://127.0.0.1:0", StaticDir: filepath.Join(t.TempDir(), "missing"), Logger: slog.Default()}, http.NotFoundHandler())
	listener, _, err := server.Listen()
	if err == nil {
		_ = listener.Close()
		t.Fatal("Listen() error = nil, want invalid static dir error")
	}
}

func TestServerListenAcceptsStaticDirWithIndex(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}
	server := New(Config{ServerUrl: "http://127.0.0.1:0", StaticDir: staticDir, Logger: slog.Default()}, http.NotFoundHandler())
	listener, _, err := server.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()
	if _, ok := listener.Addr().(*net.TCPAddr); !ok {
		t.Fatalf("listener addr = %T, want TCP", listener.Addr())
	}
}

func TestServerLogsUnifiedBackendRequests(t *testing.T) {
	var logBuffer bytes.Buffer
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"gateway"}`))
	})
	server := New(Config{Logger: slog.New(slog.NewJSONHandler(&logBuffer, nil)), RequestBodyLimit: 4096, ResponseBodyLimit: 4096}, apiHandler)

	request := httptest.NewRequest(http.MethodGet, "/api/health?x=1", nil)
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
	assertLogValue(t, started, "path", "/api/health")
	assertLogValue(t, started, "query", "x=1")
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
