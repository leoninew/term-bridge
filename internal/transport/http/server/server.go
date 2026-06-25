package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "termbridge-go/internal/infrastructure/errors"
	"termbridge-go/internal/transport/http/middleware/requestlog"
)

type Config struct {
	ServerUrl         string
	Logger            *slog.Logger
	RequestBodyLimit  int
	ResponseBodyLimit int
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
	mux.Handle("/api", apiHandler)
	mux.Handle("/api/", apiHandler)
	return &Server{config: config, server: &http.Server{Handler: requestlog.Middleware(config.Logger, requestlog.Config{RequestBodyLimit: config.RequestBodyLimit, ResponseBodyLimit: config.ResponseBodyLimit})(mux)}}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.server.Handler.ServeHTTP(w, r)
}

func (s *Server) Listen() (net.Listener, Info, error) {
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
		config.ServerUrl = "http://127.0.0.1:9010"
	}
	config.ServerUrl = strings.TrimRight(strings.TrimSpace(config.ServerUrl), "/")
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
