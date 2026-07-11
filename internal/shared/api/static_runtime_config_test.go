package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	browserdto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/browser"
)

func TestStaticHandlerInjectsRuntimeConfigOnlyIntoHTML(t *testing.T) {
	staticDir := t.TempDir()
	index := "<!doctype html><head><!-- __RUNTIME_CONFIG__ --></head><body>app</body>"
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte(index), 0o644); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "app.js"), []byte("const app = true"), 0o644); err != nil {
		t.Fatalf("WriteFile(asset) error = %v", err)
	}
	runtimeConfig := browserdto.RuntimeConfig{
		Local: browserdto.RuntimeLocalConfig{Mode: "local", PublicUrl: "http://localhost:9030", ApiBaseUrl: "/api"},
		Cloud: browserdto.RuntimeCloudConfig{PublicUrl: "https://cloud.example.com</script><script>alert(1)</script>", ApiBaseUrl: "/api"},
	}
	if err := ValidateStaticDir(staticDir, runtimeConfig); err != nil {
		t.Fatalf("ValidateStaticDir() error = %v", err)
	}

	handler := StaticHandler(staticDir, runtimeConfig)
	entry := httptest.NewRecorder()
	handler.ServeHTTP(entry, httptest.NewRequest(http.MethodGet, "/sessions", nil))
	if entry.Code != http.StatusOK {
		t.Fatalf("GET /sessions = %d", entry.Code)
	}
	if got := entry.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if strings.Contains(entry.Body.String(), runtimeConfig.Cloud.PublicUrl) {
		t.Fatalf("injected HTML contains unescaped script boundary: %s", entry.Body.String())
	}
	if !strings.Contains(entry.Body.String(), "window.__CONFIG__") || !strings.Contains(entry.Body.String(), `</script>`) {
		t.Fatalf("injected HTML = %s", entry.Body.String())
	}

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if asset.Code != http.StatusOK || asset.Body.String() != "const app = true" {
		t.Fatalf("GET /app.js = %d %q", asset.Code, asset.Body.String())
	}

	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("HEAD / = %d body=%q cache=%q", head.Code, head.Body.String(), head.Header().Get("Cache-Control"))
	}
}

func TestValidateStaticDirRejectsInvalidRuntimeConfigMarker(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}

	if err := ValidateStaticDir(staticDir, browserdto.RuntimeConfig{}); err == nil {
		t.Fatal("ValidateStaticDir() succeeded without runtime config marker")
	}
}
