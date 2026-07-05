package app

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	transportmiddleware "termbridge-go/internal/shared/api/middleware"
	"termbridge-go/internal/shared/api/middleware/requestlog"
)

func backendHandler(cfg Config, logger *slog.Logger, apiHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	agentAPIHandler := transportmiddleware.CorsForPaths(cfg.Server.CorsAllowedOrigins, apiPath)(apiHandler)
	mux.Handle("/agent-api", agentAPIHandler)
	mux.Handle("/agent-api/", agentAPIHandler)
	if cfg.Server.StaticDir != "" {
		mux.Handle("/", staticHandler(cfg.Server.StaticDir, cfg.Server.ApiBaseUrl))
	}
	return requestlog.Middleware(logger, requestlog.Config{RequestBodyLimit: cfg.LogHTTP.RequestBodyLimit, ResponseBodyLimit: cfg.LogHTTP.ResponseBodyLimit})(mux)
}

func validateStaticDir(staticDir string) error {
	if staticDir == "" {
		return nil
	}
	info, err := os.Stat(staticDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrInvalid
	}
	indexPath := filepath.Join(staticDir, "index.html")
	indexInfo, err := os.Stat(indexPath)
	if err != nil {
		return err
	}
	if indexInfo.IsDir() {
		return os.ErrInvalid
	}
	return nil
}

func staticHandler(staticDir string, apiBaseURL string) http.Handler {
	fileServer := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		urlPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if urlPath == "." {
			urlPath = ""
		}
		if urlPath != "" {
			fullPath := filepath.Join(staticDir, filepath.FromSlash(urlPath))
			if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
				if filepath.Clean(fullPath) == filepath.Clean(filepath.Join(staticDir, "index.html")) {
					serveIndexHTML(w, r, fullPath, apiBaseURL)
					return
				}
				fileServer.ServeHTTP(w, r)
				return
			}
			if path.Ext(urlPath) != "" {
				http.NotFound(w, r)
				return
			}
		}
		serveIndexHTML(w, r, filepath.Join(staticDir, "index.html"), apiBaseURL)
	})
}

func serveIndexHTML(w http.ResponseWriter, r *http.Request, indexPath string, apiBaseURL string) {
	content, err := os.ReadFile(indexPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(injectRuntimeConfig(content, apiBaseURL))
}

func injectRuntimeConfig(content []byte, apiBaseURL string) []byte {
	configValue := map[string]string{"frontendMode": "agent"}
	configValue["agentApiBaseUrl"] = normalizeRuntimeAPIBaseURL(apiBaseURL, "/agent-api")
	configJSON, err := json.Marshal(configValue)
	if err != nil {
		panic("marshal runtime config: " + err.Error())
	}
	script := []byte("<script>window.__CONFIG__ = " + string(configJSON) + ";</script>")
	placeholder := []byte("<!-- __RUNTIME_CONFIG__ -->")
	if bytes.Contains(content, placeholder) {
		return bytes.Replace(content, placeholder, script, 1)
	}
	headEnd := []byte("</head>")
	if bytes.Contains(content, headEnd) {
		return bytes.Replace(content, headEnd, append(script, headEnd...), 1)
	}
	return content
}

func normalizeRuntimeAPIBaseURL(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func apiPath(path string) bool {
	return path == "/agent-api" || strings.HasPrefix(path, "/agent-api/")
}
