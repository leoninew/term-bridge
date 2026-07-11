package app

import (
	"log/slog"
	"net/http"
	"strings"

	sharedapi "gitee.com/leoninew/TermBridge-go/internal/shared/api"
	transportmiddleware "gitee.com/leoninew/TermBridge-go/internal/shared/api/middleware"
	"gitee.com/leoninew/TermBridge-go/internal/shared/api/middleware/requestlog"
)

func backendHandler(cfg Config, logger *slog.Logger, apiHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	cloudAPIHandler := transportmiddleware.CorsForPaths(cfg.Server.CorsAllowedOrigins, apiPath)(apiHandler)
	mux.Handle("/api", cloudAPIHandler)
	mux.Handle("/api/", cloudAPIHandler)
	if cfg.Server.StaticDir != "" {
		mux.Handle("/", sharedapi.StaticHandler(cfg.Server.StaticDir, cfg.Server.BrowserRuntimeConfig))
	}
	return requestlog.Middleware(logger, cfg.LogHTTP)(mux)
}

func validateStaticDir(cfg ServerConfig) error {
	return sharedapi.ValidateStaticDir(cfg.StaticDir, cfg.BrowserRuntimeConfig)
}

func apiPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}
