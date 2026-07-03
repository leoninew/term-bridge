package server

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "termbridge-go/internal/shared/errors"
)

type Config struct {
	ServerUrl string
}

type Server struct {
	config Config
	server *http.Server
}

type Info struct {
	Url string
}

func New(config Config, handler http.Handler) *Server {
	config = normalizeConfig(config)
	if handler == nil {
		handler = http.NotFoundHandler()
	}
	return &Server{config: config, server: &http.Server{Handler: handler}}
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
		config.ServerUrl = "http://127.0.0.1:9030"
	}
	config.ServerUrl = strings.TrimRight(strings.TrimSpace(config.ServerUrl), "/")
	return config
}
