package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hashedAssetCacheControl = "public, max-age=31536000, immutable"
	otherStaticCacheControl = "public, max-age=86400"
	spaHTMLCacheControl     = "no-store"
	minCompressBytes        = 1024
)

var compressibleExt = map[string]struct{}{
	".css":  {},
	".htm":  {},
	".html": {},
	".js":   {},
	".json": {},
	".map":  {},
	".mjs":  {},
	".cjs":  {},
	".svg":  {},
	".txt":  {},
	".wasm": {},
	".xml":  {},
	".md":   {},
}

type gzipCacheKey struct {
	path    string
	modUnix int64
	size    int64
}

type gzipCacheEntry struct {
	body []byte
}

var gzipBodyCache sync.Map // gzipCacheKey -> gzipCacheEntry

func cacheControlForStaticURLPath(urlPath string) string {
	clean := path.Clean("/" + urlPath)
	if strings.HasPrefix(clean, "/assets/") {
		// Vite emits content-hashed filenames under /assets/.
		return hashedAssetCacheControl
	}
	return otherStaticCacheControl
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		token := strings.TrimSpace(strings.Split(part, ";")[0])
		if strings.EqualFold(token, "gzip") {
			return true
		}
	}
	return false
}

func shouldCompressStatic(urlPath string, size int64) bool {
	if size < minCompressBytes {
		return false
	}
	ext := strings.ToLower(path.Ext(urlPath))
	_, ok := compressibleExt[ext]
	return ok
}

func contentTypeForPath(urlPath string) string {
	ext := path.Ext(urlPath)
	if ext == "" {
		return "application/octet-stream"
	}
	if ctype := mime.TypeByExtension(ext); ctype != "" {
		return ctype
	}
	switch strings.ToLower(ext) {
	case ".js", ".mjs", ".cjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json", ".map":
		return "application/json"
	case ".wasm":
		return "application/wasm"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, fullPath, urlPath string, info os.FileInfo) {
	// Range + gzip is awkward; keep identity for partial reads.
	useGzip := r.Header.Get("Range") == "" &&
		acceptsGzip(r) &&
		shouldCompressStatic(urlPath, info.Size())

	modTime := info.ModTime()
	etag := staticETag(info, useGzip)

	w.Header().Set("Cache-Control", cacheControlForStaticURLPath(urlPath))
	w.Header().Set("Content-Type", contentTypeForPath(urlPath))
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	if useGzip {
		w.Header().Set("Vary", "Accept-Encoding")
	}

	if checkStaticNotModified(w, r, etag, modTime) {
		return
	}

	if r.Header.Get("Range") != "" {
		http.ServeFile(w, r, fullPath)
		return
	}

	if useGzip {
		if precompressed := fullPath + ".gz"; fileExists(precompressed) {
			servePrecompressedGzip(w, r, precompressed)
			return
		}
		if body, ok := loadOrBuildGzipBody(fullPath, info); ok {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(http.StatusOK)
			if r.Method == http.MethodHead {
				return
			}
			_, _ = w.Write(body)
			return
		}
	}

	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	f, err := os.Open(fullPath)
	if err != nil {
		http.Error(w, "file unavailable", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
}

func servePrecompressedGzip(w http.ResponseWriter, r *http.Request, gzPath string) {
	info, err := os.Stat(gzPath)
	if err != nil {
		http.Error(w, "file unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	f, err := os.Open(gzPath)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
}

func loadOrBuildGzipBody(fullPath string, info os.FileInfo) ([]byte, bool) {
	key := gzipCacheKey{
		path:    fullPath,
		modUnix: info.ModTime().UnixNano(),
		size:    info.Size(),
	}
	if cached, ok := gzipBodyCache.Load(key); ok {
		return cached.(gzipCacheEntry).body, true
	}

	raw, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, false
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, false
	}
	if _, err := zw.Write(raw); err != nil {
		_ = zw.Close()
		return nil, false
	}
	if err := zw.Close(); err != nil {
		return nil, false
	}
	body := buf.Bytes()
	// Only cache/serve gzip when it actually shrinks the payload.
	if len(body) >= len(raw) {
		return nil, false
	}
	gzipBodyCache.Store(key, gzipCacheEntry{body: body})
	return body, true
}

func staticETag(info os.FileInfo, gzipped bool) string {
	tag := strconv.FormatInt(info.ModTime().UnixNano(), 16) + "-" + strconv.FormatInt(info.Size(), 16)
	if gzipped {
		tag += "-gzip"
	}
	return `"` + tag + `"`
}

func checkStaticNotModified(w http.ResponseWriter, r *http.Request, etag string, modTime time.Time) bool {
	if match := r.Header.Get("If-None-Match"); match != "" {
		for _, candidate := range strings.Split(match, ",") {
			if strings.TrimSpace(candidate) == etag {
				w.WriteHeader(http.StatusNotModified)
				return true
			}
		}
	}
	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil && !modTime.After(t.Add(time.Second)) {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func writeHTMLResponse(w http.ResponseWriter, r *http.Request, content []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", spaHTMLCacheControl)

	useGzip := acceptsGzip(r) && int64(len(content)) >= minCompressBytes
	if useGzip {
		var buf bytes.Buffer
		zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		if err == nil {
			if _, err := zw.Write(content); err == nil && zw.Close() == nil && buf.Len() < len(content) {
				body := buf.Bytes()
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Vary", "Accept-Encoding")
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				w.WriteHeader(http.StatusOK)
				if r.Method == http.MethodHead {
					return
				}
				_, _ = w.Write(body)
				return
			}
		}
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(content)
}
