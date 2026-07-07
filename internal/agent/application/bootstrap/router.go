package app

import (
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	transportmiddleware "gitee.com/leoninew/TermBridge-go/internal/shared/api/middleware"
	"gitee.com/leoninew/TermBridge-go/internal/shared/api/middleware/requestlog"
)

func backendHandler(cfg Config, logger *slog.Logger, apiHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	agentAPIHandler := transportmiddleware.CorsForPaths(cfg.Server.CorsAllowedOrigins, apiPath)(apiHandler)
	mux.Handle("/agent-api", agentAPIHandler)
	mux.Handle("/agent-api/", agentAPIHandler)
	if cfg.Server.StaticDir != "" {
		mux.Handle("/", staticHandler(cfg.Server.StaticDir))
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

func staticHandler(staticDir string) http.Handler {
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
				fileServer.ServeHTTP(w, r)
				return
			}
			if path.Ext(urlPath) != "" {
				http.NotFound(w, r)
				return
			}
		}
		serveIndexHTML(w, r, filepath.Join(staticDir, "index.html"))
	})
}

func serveIndexHTML(w http.ResponseWriter, r *http.Request, indexPath string) {
	content, err := os.ReadFile(indexPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(content)
}

func apiPath(path string) bool {
	return path == "/agent-api" || strings.HasPrefix(path, "/agent-api/")
}
