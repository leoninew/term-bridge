package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	apperrors "termbridge-go/internal/infrastructure/errors"
	transportmiddleware "termbridge-go/internal/transport/http/middleware"
	"termbridge-go/internal/transport/http/middleware/requestlog"
)

type Config struct {
	ServerUrl          string
	StaticDir          string
	APIBaseURL         string
	CORSAllowedOrigins []string
	Logger             *slog.Logger
	RequestBodyLimit   int
	ResponseBodyLimit  int
}

type Server struct {
	config Config
	server *http.Server
}

type Info struct {
	Url string
}

func New(config Config, apiHandler http.Handler) *Server {
	config = normalizeConfig(config)
	mux := http.NewServeMux()
	apiHandler = transportmiddleware.CORS(config.CORSAllowedOrigins)(apiHandler)
	mux.Handle("/api", apiHandler)
	mux.Handle("/api/", apiHandler)
	if config.StaticDir != "" {
		mux.Handle("/", staticHandler(config.StaticDir, config.APIBaseURL))
	}
	return &Server{config: config, server: &http.Server{Handler: requestlog.Middleware(config.Logger, requestlog.Config{RequestBodyLimit: config.RequestBodyLimit, ResponseBodyLimit: config.ResponseBodyLimit})(mux)}}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.server.Handler.ServeHTTP(w, r)
}

func (s *Server) Listen() (net.Listener, Info, error) {
	if err := validateStaticDir(s.config.StaticDir); err != nil {
		return nil, Info{}, apperrors.Runtime("validate static web directory", err)
	}
	parsed, err := url.Parse(s.config.ServerUrl)
	if err != nil || parsed.Host == "" {
		return nil, Info{}, apperrors.Runtime("parse backend server url", err)
	}
	listener, err := net.Listen("tcp", parsed.Host)
	if err != nil {
		return nil, Info{}, apperrors.Runtime("listen backend server", err)
	}
	return listener, Info{Url: strings.TrimRight(s.config.ServerUrl, "/")}, nil
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.Serve(listener)
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
		err := <-errCh
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func normalizeConfig(config Config) Config {
	if strings.TrimSpace(config.ServerUrl) == "" {
		config.ServerUrl = "http://127.0.0.1:9030"
	}
	config.ServerUrl = strings.TrimRight(strings.TrimSpace(config.ServerUrl), "/")
	config.StaticDir = strings.TrimSpace(config.StaticDir)
	config.APIBaseURL = strings.TrimRight(strings.TrimSpace(config.APIBaseURL), "/")
	config.CORSAllowedOrigins = cleanOrigins(config.CORSAllowedOrigins)
	if config.Logger == nil {
		panic("http server logger is required")
	}
	if config.RequestBodyLimit <= 0 {
		config.RequestBodyLimit = 0
	}
	if config.ResponseBodyLimit <= 0 {
		config.ResponseBodyLimit = 0
	}
	return config
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
	configValue := map[string]string{}
	if strings.TrimSpace(apiBaseURL) != "" {
		configValue["apiBaseUrl"] = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	}
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

func cleanOrigins(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
