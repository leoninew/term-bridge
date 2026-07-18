package api

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"io"
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
		Local: browserdto.RuntimeLocalConfig{Mode: "local", PublicUrl: "http://localhost:9030", ApiBasePath: "/api"},
		Cloud: browserdto.RuntimeCloudConfig{PublicUrl: "https://cloud.example.com</script><script>alert(1)</script>", ApiBaseUrl: "https://cloud.example.com"},
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
	if got := asset.Header().Get("Cache-Control"); got != otherStaticCacheControl {
		t.Fatalf("GET /app.js Cache-Control = %q, want %q", got, otherStaticCacheControl)
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

func TestStaticHandlerCompressesAndCachesHashedAssets(t *testing.T) {
	staticDir := t.TempDir()
	index := "<!doctype html><head><!-- __RUNTIME_CONFIG__ --></head><body>app</body>"
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte(index), 0o644); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}
	assetsDir := filepath.Join(staticDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(assets) error = %v", err)
	}
	// Highly compressible payload above minCompressBytes.
	payload := bytes.Repeat([]byte("console.log('termbridge-workbench');\n"), 80)
	assetPath := filepath.Join(assetsDir, "bootstrap-AbCdEf12.js")
	if err := os.WriteFile(assetPath, payload, 0o644); err != nil {
		t.Fatalf("WriteFile(asset) error = %v", err)
	}

	handler := StaticHandler(staticDir, browserdto.RuntimeConfig{
		Local: browserdto.RuntimeLocalConfig{Mode: "local", PublicUrl: "http://localhost:9030", ApiBasePath: "/api"},
	})

	identity := httptest.NewRecorder()
	handler.ServeHTTP(identity, httptest.NewRequest(http.MethodGet, "/assets/bootstrap-AbCdEf12.js", nil))
	if identity.Code != http.StatusOK {
		t.Fatalf("identity status = %d", identity.Code)
	}
	if got := identity.Header().Get("Cache-Control"); got != hashedAssetCacheControl {
		t.Fatalf("identity Cache-Control = %q, want %q", got, hashedAssetCacheControl)
	}
	if identity.Header().Get("Content-Encoding") != "" {
		t.Fatalf("identity Content-Encoding = %q, want empty", identity.Header().Get("Content-Encoding"))
	}
	if !bytes.Equal(identity.Body.Bytes(), payload) {
		t.Fatalf("identity body mismatch")
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/bootstrap-AbCdEf12.js", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	compressed := httptest.NewRecorder()
	handler.ServeHTTP(compressed, req)
	if compressed.Code != http.StatusOK {
		t.Fatalf("gzip status = %d", compressed.Code)
	}
	if got := compressed.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("gzip Content-Encoding = %q", got)
	}
	if got := compressed.Header().Get("Cache-Control"); got != hashedAssetCacheControl {
		t.Fatalf("gzip Cache-Control = %q, want %q", got, hashedAssetCacheControl)
	}
	if got := compressed.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("gzip Vary = %q", got)
	}
	if compressed.Body.Len() >= len(payload) {
		t.Fatalf("gzip body not smaller: %d >= %d", compressed.Body.Len(), len(payload))
	}
	gr, err := gzip.NewReader(bytes.NewReader(compressed.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	decoded, err := io.ReadAll(gr)
	_ = gr.Close()
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("decoded gzip body mismatch")
	}

	// Precompressed sibling wins when present.
	var pre bytes.Buffer
	zw := gzip.NewWriter(&pre)
	if _, err := zw.Write([]byte("precompressed-body")); err != nil {
		t.Fatalf("precompress write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("precompress close: %v", err)
	}
	if err := os.WriteFile(assetPath+".gz", pre.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(precompressed) error = %v", err)
	}
	// Drop in-memory cache entries for this path by rewriting mtime via rewrite.
	// Using a new filename avoids stale in-memory cache from the earlier payload.
	preName := "bootstrap-PreGz99.js"
	prePath := filepath.Join(assetsDir, preName)
	if err := os.WriteFile(prePath, payload, 0o644); err != nil {
		t.Fatalf("WriteFile(pre source): %v", err)
	}
	if err := os.WriteFile(prePath+".gz", pre.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(pre gz): %v", err)
	}
	preReq := httptest.NewRequest(http.MethodGet, "/assets/"+preName, nil)
	preReq.Header.Set("Accept-Encoding", "gzip")
	preRec := httptest.NewRecorder()
	handler.ServeHTTP(preRec, preReq)
	if preRec.Header().Get("Content-Encoding") != "gzip" || !bytes.Equal(preRec.Body.Bytes(), pre.Bytes()) {
		t.Fatalf("precompressed serve mismatch encoding=%q body=%q", preRec.Header().Get("Content-Encoding"), preRec.Body.Bytes())
	}

	// Not modified with matching ETag.
	etag := compressed.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag on gzip response")
	}
	nmReq := httptest.NewRequest(http.MethodGet, "/assets/bootstrap-AbCdEf12.js", nil)
	nmReq.Header.Set("Accept-Encoding", "gzip")
	nmReq.Header.Set("If-None-Match", etag)
	nm := httptest.NewRecorder()
	handler.ServeHTTP(nm, nmReq)
	if nm.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match status = %d, want 304", nm.Code)
	}
}

func TestStaticHandlerETagMatchesActualEncodingWhenGzipDoesNotShrink(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<!doctype html><!-- __RUNTIME_CONFIG__ -->"), 0o644); err != nil {
		t.Fatalf("WriteFile(index): %v", err)
	}
	assetsDir := filepath.Join(staticDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// High-entropy payload so runtime gzip does not shrink below the original size.
	payload := make([]byte, minCompressBytes+256)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	// Double-check the invariant this test relies on.
	var check bytes.Buffer
	zw, err := gzip.NewWriterLevel(&check, gzip.BestSpeed)
	if err != nil {
		t.Fatalf("gzip.NewWriterLevel: %v", err)
	}
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if check.Len() < len(payload) {
		t.Fatalf("test setup: random payload unexpectedly compressible (%d < %d)", check.Len(), len(payload))
	}
	assetName := "noise-AbCdEf12.js"
	if err := os.WriteFile(filepath.Join(assetsDir, assetName), payload, 0o644); err != nil {
		t.Fatalf("WriteFile(asset): %v", err)
	}

	handler := StaticHandler(staticDir, browserdto.RuntimeConfig{})
	req := httptest.NewRequest(http.MethodGet, "/assets/"+assetName, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty (identity)", got)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}
	if strings.Contains(etag, "-gzip") {
		t.Fatalf("ETag = %q includes -gzip for identity body", etag)
	}
	if !bytes.Equal(rec.Body.Bytes(), payload) {
		t.Fatal("identity body mismatch")
	}

	// Conditional request with the identity ETag must 304.
	nmReq := httptest.NewRequest(http.MethodGet, "/assets/"+assetName, nil)
	nmReq.Header.Set("Accept-Encoding", "gzip")
	nmReq.Header.Set("If-None-Match", etag)
	nm := httptest.NewRecorder()
	handler.ServeHTTP(nm, nmReq)
	if nm.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match status = %d, want 304", nm.Code)
	}
	if got := nm.Header().Get("ETag"); got != etag {
		t.Fatalf("304 ETag = %q, want %q", got, etag)
	}
}

func TestStaticHandlerSkipsGzipForTinyOrBinaryAssets(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<!doctype html><!-- __RUNTIME_CONFIG__ -->"), 0o644); err != nil {
		t.Fatalf("WriteFile(index): %v", err)
	}
	assetsDir := filepath.Join(staticDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "tiny.js"), []byte("x=1"), 0o644); err != nil {
		t.Fatalf("WriteFile(tiny): %v", err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	png = append(png, bytes.Repeat([]byte{1, 2, 3, 4}, 300)...)
	if err := os.WriteFile(filepath.Join(assetsDir, "logo.png"), png, 0o644); err != nil {
		t.Fatalf("WriteFile(png): %v", err)
	}

	handler := StaticHandler(staticDir, browserdto.RuntimeConfig{})

	tinyReq := httptest.NewRequest(http.MethodGet, "/assets/tiny.js", nil)
	tinyReq.Header.Set("Accept-Encoding", "gzip")
	tiny := httptest.NewRecorder()
	handler.ServeHTTP(tiny, tinyReq)
	if tiny.Header().Get("Content-Encoding") != "" || tiny.Body.String() != "x=1" {
		t.Fatalf("tiny.js unexpectedly compressed: encoding=%q body=%q", tiny.Header().Get("Content-Encoding"), tiny.Body.String())
	}

	pngReq := httptest.NewRequest(http.MethodGet, "/assets/logo.png", nil)
	pngReq.Header.Set("Accept-Encoding", "gzip")
	pngRec := httptest.NewRecorder()
	handler.ServeHTTP(pngRec, pngReq)
	if pngRec.Header().Get("Content-Encoding") != "" || !bytes.Equal(pngRec.Body.Bytes(), png) {
		t.Fatalf("png unexpectedly compressed: encoding=%q", pngRec.Header().Get("Content-Encoding"))
	}
}
