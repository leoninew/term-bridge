package localapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"

	terminalapp "termbridge-go/internal/application/terminal"
	apperrors "termbridge-go/internal/infrastructure/errors"
	"termbridge-go/internal/protocol/terminal"
)

type Config struct {
	AllowedOrigins []string
	DebugErrors    bool
	Logger         *slog.Logger
}

type Handler struct {
	config   Config
	registry *terminalapp.Registry
	logger   *slog.Logger
}

func New(config Config, registry *terminalapp.Registry) *Handler {
	config = normalizeConfig(config)
	return &Handler{config: config, registry: registry, logger: config.Logger}
}

func (h *Handler) Handler() http.Handler {
	return h.routes()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Handler().ServeHTTP(w, r)
}

func (h *Handler) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", h.handleHealth)
	mux.HandleFunc("/api/workspaces", h.withOriginGuard(h.handleWorkspaces))
	mux.HandleFunc("/api/workspaces/", h.withOriginGuard(h.handleWorkspace))
	mux.HandleFunc("/api/sessions", h.withOriginGuard(h.handleSessions))
	mux.HandleFunc("/api/sessions/", h.withOriginGuard(h.handleSession))
	return mux
}

func (s *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Handler) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/workspaces")
	if path == "" || path == "/" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		workspaces, err := s.registry.ListWorkspaces()
		if err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, workspaces)
		return
	}
	if path == "/tree" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		tree, err := s.registry.WorkspaceTree()
		if err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, tree)
		return
	}
	if path == "/order" {
		if r.Method != http.MethodPatch {
			methodNotAllowed(w)
			return
		}
		var request terminalapp.UpdateWorkspaceOrderRequest
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		workspaces, err := s.registry.UpdateWorkspaceOrder(request.WorkspaceIds)
		if err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, workspaces)
		return
	}
	apiNotFound(w)
}

func (s *Handler) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/workspaces/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		apiNotFound(w)
		return
	}
	if len(parts) == 1 && parts[0] == "tree" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		tree, err := s.registry.WorkspaceTree()
		if err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, tree)
		return
	}
	if len(parts) == 1 && parts[0] == "order" {
		if r.Method != http.MethodPatch {
			methodNotAllowed(w)
			return
		}
		var request terminalapp.UpdateWorkspaceOrderRequest
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		workspaces, err := s.registry.UpdateWorkspaceOrder(request.WorkspaceIds)
		if err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, workspaces)
		return
	}
	workspaceId := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodDelete {
			methodNotAllowed(w)
			return
		}
		if err := s.registry.DeleteWorkspace(workspaceId); err != nil {
			s.writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 2 && parts[1] == "sessions" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		sessions, err := s.registry.ListSessionsByWorkspaceId(workspaceId)
		if err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sessions)
		return
	}
	apiNotFound(w)
}

func (s *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sessions, err := s.registry.ListSessions()
		if err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sessions)
	case http.MethodPost:
		var request terminalapp.CreateSessionRequest
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		response, err := s.registry.CreateSession(r.Context(), request)
		if err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w)
	}
}

func (s *Handler) handleSession(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		apiNotFound(w)
		return
	}
	sessionId := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			summary, err := s.registry.GetSession(sessionId)
			if err != nil {
				s.writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, summary)
		case http.MethodPatch:
			var request terminalapp.UpdateSessionRequest
			if !s.decodeJSONRequest(w, r, &request) {
				return
			}
			summary, err := s.registry.UpdateSession(sessionId, request)
			if err != nil {
				s.writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, summary)
		case http.MethodDelete:
			if err := s.registry.DeleteSession(sessionId); err != nil {
				s.writeError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w)
		}
		return
	}
	if len(parts) != 2 {
		apiNotFound(w)
		return
	}
	switch parts[1] {
	case "close":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		summary, err := s.registry.CloseSession(sessionId, "api_close")
		if err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
	case "history":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		data, err := s.registry.History(sessionId)
		if err != nil {
			s.writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	case "ws":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleWebSocket(w, r, sessionId)
	default:
		apiNotFound(w)
	}
}

func (s *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request, sessionId string) {
	client, err := s.registry.Attach(sessionId)
	if err != nil {
		s.writeError(w, err)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{terminalproto.Subprotocol}, OriginPatterns: s.originPatterns(r)})
	if err != nil {
		s.logger.Info("terminal websocket accept failed", "session_id", sessionId, "client_id", client.ID(), "error", err)
		client.Detach("websocket_accept_failed")
		return
	}
	s.logger.Info("terminal websocket accepted", "session_id", sessionId, "client_id", client.ID(), "remote", r.RemoteAddr)
	conn.SetReadLimit(terminalproto.MaxBinaryFrameBytes)
	defer conn.Close(websocket.StatusNormalClosure, "closed")
	s.serveWebSocket(r.Context(), conn, client)
}

func (s *Handler) serveWebSocket(ctx context.Context, conn *websocket.Conn, client *terminalapp.Client) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for outbound := range client.Outbound() {
			writeCtx, writeCancel := context.WithTimeout(ctx, 10*time.Second)
			var err error
			switch outbound.Kind {
			case terminalapp.OutboundBinary:
				err = conn.Write(writeCtx, websocket.MessageBinary, outbound.Binary)
				if err == nil {
					chunks, totalBytes := client.MarkBinarySent(len(outbound.Binary))
					if chunks <= 5 || chunks%50 == 0 {
						s.logger.Info("terminal websocket binary sent", "session_id", client.SessionID(), "client_id", client.ID(), "chunk_bytes", len(outbound.Binary), "chunks", chunks, "total_bytes", totalBytes, "queued_bytes", client.QueuedBytes())
					}
				}
			case terminalapp.OutboundText:
				data, encodeErr := terminalproto.EncodeServer(outbound.Text)
				if encodeErr != nil {
					err = encodeErr
				} else {
					err = conn.Write(writeCtx, websocket.MessageText, data)
				}
			}
			writeCancel()
			client.MarkSent(outbound)
			if err != nil {
				s.logger.Info("terminal websocket write failed", "session_id", client.SessionID(), "client_id", client.ID(), "kind", outbound.Kind, "error", err)
				client.Detach("websocket_write_failed")
				return
			}
		}
	}()
	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			s.logger.Info("terminal websocket read closed", "session_id", client.SessionID(), "client_id", client.ID(), "error", err)
			client.Detach("client_disconnected")
			cancel()
			<-writerDone
			return
		}
		switch messageType {
		case websocket.MessageBinary:
			if len(data) > terminalproto.MaxBinaryFrameBytes {
				client.SendControl(s.protocolError("binary_frame_too_large", "binary frame too large", nil))
				client.Detach("binary_frame_too_large")
				cancel()
				<-writerDone
				return
			}
			if err := client.WriteInput(data); err != nil {
				client.SendControl(s.protocolError("write_failed", "write failed", err))
			}
		case websocket.MessageText:
			message, err := terminalproto.DecodeClient(data)
			if err != nil {
				client.SendControl(s.protocolError("invalid_control", "invalid control message", err))
				continue
			}
			if !s.handleControl(client, message) {
				cancel()
				<-writerDone
				return
			}
		}
	}
}

func (s *Handler) handleControl(client *terminalapp.Client, message terminalproto.ClientMessage) bool {
	switch message.Type {
	case terminalproto.TypeHello:
		return true
	case terminalproto.TypeResize:
		s.logger.Info("terminal resize received", "session_id", client.SessionID(), "client_id", client.ID(), "cols", message.Cols, "rows", message.Rows)
		if err := client.Resize(message.Cols, message.Rows); err != nil {
			client.SendControl(s.protocolError("resize_failed", "resize failed", err))
		}
		return true
	case terminalproto.TypeDetach:
		client.Detach("client_detached")
		return false
	case terminalproto.TypePing:
		client.SendControl(terminalproto.ServerMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
		return true
	default:
		client.SendControl(s.protocolError("unknown_control", "unknown control message", fmt.Errorf("unknown control %q", message.Type)))
		return true
	}
}

func (s *Handler) withOriginGuard(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.originAllowed(r) {
			writeAPIError(w, http.StatusForbidden, "forbidden_origin", "origin is not allowed", "")
			return
		}
		handler(w, r)
	}
}

func (s *Handler) originAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	if origin == "http://"+r.Host || origin == "https://"+r.Host {
		return true
	}
	for _, allowed := range s.config.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func (s *Handler) originPatterns(r *http.Request) []string {
	patterns := []string{"http://" + r.Host, "https://" + r.Host}
	patterns = append(patterns, s.config.AllowedOrigins...)
	return patterns
}

type apiErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

func (s *Handler) decodeJSONRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, terminalproto.MaxJSONMessageBytes))
	if err := decoder.Decode(value); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request", s.debugError(err))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Handler) writeError(w http.ResponseWriter, err error) {
	status, code := statusCodeForError(err)
	writeAPIError(w, status, code, apperrors.Message(err), s.debugError(err))
}

func statusCodeForError(err error) (int, string) {
	switch apperrors.KindOf(err) {
	case apperrors.KindNotFound:
		return http.StatusNotFound, "not_found"
	case apperrors.KindUsage:
		return http.StatusBadRequest, "usage_error"
	case apperrors.KindConfig:
		return http.StatusBadRequest, "config_error"
	case apperrors.KindRuntime:
		return http.StatusInternalServerError, "runtime_error"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

func writeAPIError(w http.ResponseWriter, status int, code string, message string, debug string) {
	writeJSON(w, status, apiErrorResponse{Code: code, Message: message, Error: debug})
}

func (s *Handler) debugError(err error) string {
	if !s.config.DebugErrors {
		return ""
	}
	return apperrors.Debug(err)
}

func (s *Handler) protocolError(code string, message string, err error) terminalproto.ServerMessage {
	return terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: code, Message: message, Error: s.debugError(err)}
}

func apiNotFound(w http.ResponseWriter) {
	writeAPIError(w, http.StatusNotFound, "not_found", "not found", "")
}

func methodNotAllowed(w http.ResponseWriter) {
	writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", "")
}

func normalizeConfig(config Config) Config {
	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return config
}
