package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	apperrors "termbridge-go/internal/infrastructure/errors"
	"termbridge-go/internal/transport/http/middleware/requestlog"
)

type Config struct {
	Host              string
	Port              int
	Open              bool
	Dev               bool
	Logger            *slog.Logger
	RequestBodyLimit  int
	ResponseBodyLimit int
}

type Server struct {
	config Config
	server *http.Server
}

type Info struct {
	URL string
}

func New(config Config, localHandler http.Handler, gatewayHandler http.Handler) *Server {
	config = normalizeConfig(config)
	mux := http.NewServeMux()
	mux.Handle("/api/gateway/", gatewayHandler)
	mux.Handle("/api", localHandler)
	mux.Handle("/api/", localHandler)
	return &Server{config: config, server: &http.Server{Handler: requestlog.Middleware(config.Logger, requestlog.Config{RequestBodyLimit: config.RequestBodyLimit, ResponseBodyLimit: config.ResponseBodyLimit})(mux)}}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.server.Handler.ServeHTTP(w, r)
}

func (s *Server) Listen() (net.Listener, Info, error) {
	addr := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, Info{}, apperrors.Runtime("listen backend server", err)
	}
	info := Info{URL: "http://" + listener.Addr().String()}
	if s.config.Open {
		go openBrowser(info.URL)
	}
	return listener, info, nil
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
	if strings.TrimSpace(config.Host) == "" {
		config.Host = "127.0.0.1"
	}
	if config.Port == 0 {
		config.Port = 9010
	}
	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if config.RequestBodyLimit <= 0 {
		config.RequestBodyLimit = 4096
	}
	if config.ResponseBodyLimit <= 0 {
		config.ResponseBodyLimit = 4096
	}
	return config
}

func openBrowser(url string) {
	commands := [][]string{{"open", url}, {"xdg-open", url}, {"rundll32", "url.dll,FileProtocolHandler", url}}
	for _, command := range commands {
		if _, err := exec.LookPath(command[0]); err != nil {
			continue
		}
		_ = exec.Command(command[0], command[1:]...).Start()
		return
	}
}
