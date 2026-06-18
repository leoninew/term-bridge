package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	apperrors "termbridge-go/internal/errors"
	"termbridge-go/internal/terminalproto"
	"termbridge-go/internal/webterminal"
)

type Config struct {
	Host              string
	Port              int
	Dev               bool
	Open              bool
	Logger            *slog.Logger
	RequestBodyLimit  int
	ResponseBodyLimit int
}

type Server struct {
	config   Config
	registry *webterminal.Registry
	server   *http.Server
}

type Info struct {
	URL string
}

func New(config Config, registry *webterminal.Registry) *Server {
	mux := http.NewServeMux()
	s := &Server{config: normalizeConfig(config), registry: registry}
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/workspaces", s.handleWorkspaces)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSession)
	s.server = &http.Server{Handler: logRequests(s.config.Logger, s.config.RequestBodyLimit, s.config.ResponseBodyLimit, mux)}
	return s
}

func (s *Server) Listen() (net.Listener, Info, error) {
	addr := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, Info{}, apperrors.Runtime("listen web server", err)
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

func (s *Server) ListenAndServe(ctx context.Context) (Info, error) {
	listener, info, err := s.Listen()
	if err != nil {
		return Info{}, err
	}
	return info, s.Serve(ctx, listener)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	workspaces, err := s.registry.ListWorkspaces()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sessions, err := s.registry.ListSessions()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	case http.MethodPost:
		var request webterminal.CreateSessionRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, terminalproto.MaxJSONMessageBytes))
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody("invalid_request", err.Error()))
			return
		}
		response, err := s.registry.CreateSession(r.Context(), request)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	sessionID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		summary, err := s.registry.GetSession(sessionID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	switch parts[1] {
	case "history":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		data, err := s.registry.History(sessionID)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	case "ws":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleWebSocket(w, r, sessionID)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request, sessionID string) {
	client, err := s.registry.Attach(sessionID)
	if err != nil {
		writeError(w, err)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{terminalproto.Subprotocol}, InsecureSkipVerify: true})
	if err != nil {
		client.Detach("websocket_accept_failed")
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closed")
	s.serveWebSocket(r.Context(), conn, client)
}

func (s *Server) serveWebSocket(ctx context.Context, conn *websocket.Conn, client *webterminal.Client) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for outbound := range client.Outbound() {
			writeCtx, writeCancel := context.WithTimeout(ctx, 10*time.Second)
			var err error
			switch outbound.Kind {
			case webterminal.OutboundBinary:
				err = conn.Write(writeCtx, websocket.MessageBinary, outbound.Binary)
			case webterminal.OutboundText:
				data, encodeErr := terminalproto.EncodeServer(outbound.Text)
				if encodeErr != nil {
					err = encodeErr
				} else {
					err = conn.Write(writeCtx, websocket.MessageText, data)
				}
			}
			writeCancel()
			if err != nil {
				client.Detach("websocket_write_failed")
				return
			}
		}
	}()
	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			client.Detach("client_disconnected")
			cancel()
			<-writerDone
			return
		}
		switch messageType {
		case websocket.MessageBinary:
			if err := client.WriteInput(data); err != nil {
				writeControl(ctx, conn, terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "write_failed", Message: err.Error()})
			}
		case websocket.MessageText:
			message, err := terminalproto.DecodeClient(data)
			if err != nil {
				writeControl(ctx, conn, terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "invalid_control", Message: err.Error()})
				continue
			}
			if !handleControl(ctx, conn, client, message) {
				cancel()
				<-writerDone
				return
			}
		}
	}
}

func handleControl(ctx context.Context, conn *websocket.Conn, client *webterminal.Client, message terminalproto.ClientMessage) bool {
	switch message.Type {
	case terminalproto.TypeHello:
		return true
	case terminalproto.TypeResize:
		if err := client.Resize(message.Cols, message.Rows); err != nil {
			writeControl(ctx, conn, terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "resize_failed", Message: err.Error()})
		}
		return true
	case terminalproto.TypeDetach:
		client.Detach("client_detached")
		return false
	case terminalproto.TypeClose:
		if err := client.CloseSession(); err != nil {
			writeControl(ctx, conn, terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "close_failed", Message: err.Error()})
		}
		return false
	case terminalproto.TypePing:
		writeControl(ctx, conn, terminalproto.ServerMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
		return true
	default:
		writeControl(ctx, conn, terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "unknown_control", Message: fmt.Sprintf("unknown control %q", message.Type)})
		return true
	}
}

func writeControl(ctx context.Context, conn *websocket.Conn, message terminalproto.ServerMessage) {
	data, err := terminalproto.EncodeServer(message)
	if err != nil {
		return
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = conn.Write(writeCtx, websocket.MessageText, data)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "runtime_error"
	if apperrors.IsUsage(err) {
		status = http.StatusBadRequest
		code = "usage_error"
	} else if apperrors.IsConfig(err) {
		status = http.StatusBadRequest
		code = "config_error"
	}
	writeJSON(w, status, errorBody(code, apperrors.FormatUser(err)))
}

func errorBody(code string, message string) map[string]any {
	return map[string]any{"error": map[string]string{"code": code, "message": message}}
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, errorBody("method_not_allowed", "method not allowed"))
}

func normalizeConfig(config Config) Config {
	if strings.TrimSpace(config.Host) == "" {
		config.Host = "127.0.0.1"
	}
	if config.Port < 0 || config.Port > 65535 {
		config.Port = 0
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
	commands := [][]string{{"rundll32", "url.dll,FileProtocolHandler", url}, {"cmd", "/C", "start", "", url}}
	for _, command := range commands {
		if err := exec.Command(command[0], command[1:]...).Start(); err == nil {
			return
		}
	}
}
