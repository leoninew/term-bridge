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

	transportmiddleware "gitee.com/leoninew/TermBridge-go/internal/shared/api/middleware"
	"gitee.com/leoninew/TermBridge-go/internal/shared/api/middleware/requestlog"
)

func backendHandler(cfg Config, logger *slog.Logger, apiHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	agentAPIHandler := transportmiddleware.CorsForPaths(cfg.Server.CorsAllowedOrigins, apiPath)(apiHandler)
	mux.Handle("/agent-api", agentAPIHandler)
	mux.Handle("/agent-api/", agentAPIHandler)
	if cfg.Server.StaticDir != "" {
		mux.Handle("/", staticHandler(cfg.Server.StaticDir, runtimeConfig{
			FrontendMode:    "agent",
			AgentApiBaseUrl: normalizeRuntimeAPIBaseURL(cfg.Server.ApiBaseUrl, "/agent-api"),
			CloudApiBaseUrl: cloudRuntimeApiBaseURL(cfg.Cloud.PublicURL),
			CloudOAuth:      cloudRuntimeOAuthConfig(cfg.Cloud.OAuthClient),
		}))
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

func staticHandler(staticDir string, config runtimeConfig) http.Handler {
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
					serveIndexHTML(w, r, fullPath, config)
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
		serveIndexHTML(w, r, filepath.Join(staticDir, "index.html"), config)
	})
}

type runtimeConfig struct {
	FrontendMode    string              `json:"frontendMode"`
	AgentApiBaseUrl string              `json:"agentApiBaseUrl"`
	CloudApiBaseUrl string              `json:"cloudApiBaseUrl,omitempty"`
	CloudOAuth      *runtimeOAuthConfig `json:"cloudOAuth,omitempty"`
}

type runtimeOAuthConfig struct {
	ClientId    string   `json:"clientId,omitempty"`
	RedirectUrl string   `json:"redirectUrl,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
}

func serveIndexHTML(w http.ResponseWriter, r *http.Request, indexPath string, config runtimeConfig) {
	content, err := os.ReadFile(indexPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(injectRuntimeConfig(content, config))
}

func injectRuntimeConfig(content []byte, config runtimeConfig) []byte {
	configJSON, err := json.Marshal(config)
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

func cloudRuntimeApiBaseURL(publicURL string) string {
	if strings.TrimSpace(publicURL) == "" {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(publicURL), "/") + "/cloud-api"
}

func cloudRuntimeOAuthConfig(config OAuthClientConfig) *runtimeOAuthConfig {
	if strings.TrimSpace(config.ClientId) == "" || strings.TrimSpace(config.RedirectUrl) == "" {
		return nil
	}
	return &runtimeOAuthConfig{ClientId: strings.TrimSpace(config.ClientId), RedirectUrl: strings.TrimSpace(config.RedirectUrl), Scopes: cleanRuntimeOAuthScopes(config.Scopes)}
}

func cleanRuntimeOAuthScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		value := strings.TrimSpace(scope)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func apiPath(path string) bool {
	return path == "/agent-api" || strings.HasPrefix(path, "/agent-api/")
}
