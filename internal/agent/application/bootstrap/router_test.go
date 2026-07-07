package app

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackendHandlerServesStaticFilesWithSPAFallbackAndKeepsAPIRoutes(t *testing.T) {
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
		case "/agent-api/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"agent"}`))
		default:
			http.NotFound(w, r)
		}
	})
	handler := backendHandler(Config{Server: ServerConfig{StaticDir: staticDir}}, slog.Default(), apiHandler)

	cases := []struct {
		path string
		want string
	}{
		{path: "/", want: "<html>app</html>"},
		{path: "/sessions", want: "<html>app</html>"},
		{path: "/settings", want: "<html>app</html>"},
		{path: "/assets/app.js", want: "console.log('app')"},
		{path: "/agent-api/health", want: `{"status":"agent"}`},
	}
	for _, tc := range cases {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), tc.want) {
			t.Fatalf("GET %s = %d %q, want body containing %q", tc.path, response.Code, response.Body.String(), tc.want)
		}
	}

	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/agent-api/missing", nil))
	if apiResponse.Code != http.StatusNotFound || strings.Contains(apiResponse.Body.String(), "<html>app</html>") {
		t.Fatalf("/agent-api/missing = %d %q, want API 404 without SPA fallback", apiResponse.Code, apiResponse.Body.String())
	}
}

func TestBackendHandlerInjectsRuntimeConfigIntoIndexHTML(t *testing.T) {
	staticDir := t.TempDir()
	index := "<!doctype html><html><head><!-- __RUNTIME_CONFIG__ --><title>app</title></head><body></body></html>"
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte(index), 0o644); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}
	handler := backendHandler(Config{
		Server: ServerConfig{StaticDir: staticDir, ApiBaseUrl: "https://agent.example.com/"},
		Cloud: CloudConnectorConfig{
			PublicURL: "https://cloud.example.com/",
			OAuthClient: OAuthClientConfig{
				ClientId:    "termbridge-agent",
				RedirectUrl: "http://127.0.0.1:9033/agent/oauth/callback",
				Scopes:      []string{"openid", "email", "profile"},
			},
		},
	}, slog.Default(), http.NotFoundHandler())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("GET / = %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	want := `<script>window.__CONFIG__ = {"frontendMode":"agent","agentApiBaseUrl":"https://agent.example.com","cloudApiBaseUrl":"https://cloud.example.com/cloud-api","cloudOAuth":{"clientId":"termbridge-agent","redirectUrl":"http://127.0.0.1:9033/agent/oauth/callback","scopes":["openid","email","profile"]}};</script>`
	if !strings.Contains(response.Body.String(), want) {
		t.Fatalf("runtime config missing: %s", response.Body.String())
	}
}

func TestBackendHandlerLogsUnifiedRequests(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agent-api/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"agent"}`))
	})
	handler := backendHandler(Config{LogHTTP: LogHTTPConfig{RequestBodyLimit: 4096, ResponseBodyLimit: 4096}}, logger, apiHandler)

	request := httptest.NewRequest(http.MethodGet, "/agent-api/health?x=1", nil)
	request.Header.Set("User-Agent", "test-agent")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	entries := decodeBackendLogEntries(t, logBuffer.String())
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d: %+v", len(entries), entries)
	}
	started := entries[0]
	completed := entries[1]
	assertBackendLogValue(t, started, "msg", "request started")
	assertBackendLogValue(t, completed, "msg", "request completed")
	assertBackendLogValue(t, started, "method", http.MethodGet)
	assertBackendLogValue(t, started, "path", "/agent-api/health")
	assertBackendLogValue(t, started, "query", "x=1")
	assertBackendLogNumber(t, completed, "status", http.StatusOK)
	assertBackendLogValue(t, completed, "response_body", response.Body.String())
	if started["request_id"] == "" || started["request_id"] != completed["request_id"] {
		t.Fatalf("expected matching request_id, got started=%#v completed=%#v", started["request_id"], completed["request_id"])
	}
}

func TestBackendHandlerAppliesCorsOnlyToAPIRoutes(t *testing.T) {
	handler := backendHandler(Config{Server: ServerConfig{CorsAllowedOrigins: []string{"https://app.example.com"}}}, slog.Default(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	apiResponse := httptest.NewRecorder()
	apiRequest := httptest.NewRequest(http.MethodOptions, "/agent-api/health", nil)
	apiRequest.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusNoContent || apiResponse.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("API CORS response = %d origin=%q", apiResponse.Code, apiResponse.Header().Get("Access-Control-Allow-Origin"))
	}

	staticResponse := httptest.NewRecorder()
	staticRequest := httptest.NewRequest(http.MethodOptions, "/favicon.ico", nil)
	staticRequest.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(staticResponse, staticRequest)
	if staticResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("static CORS origin = %q, want empty", staticResponse.Header().Get("Access-Control-Allow-Origin"))
	}
}

func decodeBackendLogEntries(t *testing.T, content string) []map[string]any {
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

func assertBackendLogValue(t *testing.T, entry map[string]any, key string, want string) {
	t.Helper()
	if got, ok := entry[key].(string); !ok || got != want {
		t.Fatalf("expected %s=%q, got %#v", key, want, entry[key])
	}
}

func assertBackendLogNumber(t *testing.T, entry map[string]any, key string, want int) {
	t.Helper()
	got, ok := entry[key].(float64)
	if !ok || int(got) != want {
		t.Fatalf("expected %s=%d, got %#v", key, want, entry[key])
	}
}
