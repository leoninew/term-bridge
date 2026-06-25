package gatewayapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"termbridge-go/internal/protocol/terminal"
	"termbridge-go/internal/protocol/tunnel"
	"termbridge-go/internal/transport/http/gatewayapi/auth"
)

type Config struct {
	Username       string
	Password       string
	JWTSecret      string
	AllowedOrigins []string
	DebugErrors    bool
	Logger         *slog.Logger
}

type Handler struct {
	config             Config
	auth               *auth.Auther
	registry           *DeviceRegistry
	routes             map[string]*agentRoute
	routeMu            sync.Mutex
	writers            map[string]tunnel.StreamId
	writerMu           sync.Mutex
	cacheMu            sync.Mutex
	workspaceTreeCache map[string]json.RawMessage
	historyCache       map[string]map[string]string
}

type DeviceSummary struct {
	Id          string    `json:"id"`
	Name        string    `json:"name"`
	Online      bool      `json:"online"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
}

type DeviceRegistry struct {
	mu      sync.Mutex
	devices map[string]DeviceSummary
}

func NewDeviceRegistry() *DeviceRegistry {
	return &DeviceRegistry{devices: map[string]DeviceSummary{}}
}

func (r *DeviceRegistry) Register(id string, name string, now time.Time) DeviceSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	device := r.devices[id]
	if device.ConnectedAt.IsZero() {
		device.ConnectedAt = now
	}
	device.Id = id
	device.Name = name
	device.Online = true
	device.LastSeen = now
	r.devices[id] = device
	return device
}

func (r *DeviceRegistry) MarkOffline(id string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	device, ok := r.devices[id]
	if !ok {
		return
	}
	device.Online = false
	device.LastSeen = now
	r.devices[id] = device
}

func (r *DeviceRegistry) List() []DeviceSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	devices := make([]DeviceSummary, 0, len(r.devices))
	for _, device := range r.devices {
		devices = append(devices, device)
	}
	return devices
}

func New(config Config) *Handler {
	config = normalizeConfig(config)
	return &Handler{
		config:             config,
		auth:               auth.NewAuther(auth.Credentials{Username: config.Username, Password: config.Password}, auth.NewTokenService(config.JWTSecret)),
		registry:           NewDeviceRegistry(),
		routes:             map[string]*agentRoute{},
		writers:            map[string]tunnel.StreamId{},
		workspaceTreeCache: map[string]json.RawMessage{},
		historyCache:       map[string]map[string]string{},
	}
}

func (h *Handler) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", h.handleHealth)
	mux.HandleFunc("/api/login", h.handleLogin)
	mux.HandleFunc("/api/logout", h.auth.Middleware(http.HandlerFunc(h.handleLogout), h.writeUnauthorized).ServeHTTP)
	mux.HandleFunc("/api/me", h.auth.Middleware(http.HandlerFunc(h.handleMe), h.writeUnauthorized).ServeHTTP)
	mux.HandleFunc("/api/devices", h.auth.Middleware(http.HandlerFunc(h.handleDevices), h.writeUnauthorized).ServeHTTP)
	mux.HandleFunc("/api/devices/", h.auth.Middleware(http.HandlerFunc(h.handleDevice), h.writeUnauthorized).ServeHTTP)
	mux.HandleFunc("/api/agent/tunnel", h.handleAgentTunnel)
	mux.HandleFunc("/api", h.writeNotFound)
	mux.HandleFunc("/api/", h.writeNotFound)
	return mux
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Handler().ServeHTTP(w, r)
}

func (s *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !s.decodeJSONRequest(w, r, &request) {
		return
	}
	if !s.auth.ValidCredentials(request.Username, request.Password) {
		s.writeUnauthorized(w, r)
		return
	}
	token, err := s.auth.SignToken(request.Username)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, "Failed to sign token", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"access_token": token, "token_type": "bearer"})
}

func (s *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": s.auth.UsernameFromRequest(r)})
}

func (s *Handler) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/devices" {
		s.writeNotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, s.registry.List())
}

func (s *Handler) handleDevice(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/devices/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		s.writeNotFound(w, r)
		return
	}
	deviceId := parts[0]
	route := s.routeFor(deviceId)
	switch parts[1] {
	case "workspaces":
		s.handleWorkspaceRoute(w, r, route, deviceId, parts)
	case "sessions":
		if len(parts) == 2 && r.Method == http.MethodPost {
			var request json.RawMessage
			if !s.decodeJSONRequest(w, r, &request) {
				return
			}
			s.handleJSONRelayWithStatus(w, r, route, deviceId, "create_session", request, "", http.StatusCreated)
			return
		}
		s.writeNotFound(w, r)
	default:
		s.writeNotFound(w, r)
	}
}

func (s *Handler) handleWorkspaceRoute(w http.ResponseWriter, r *http.Request, route *agentRoute, deviceId string, parts []string) {
	if len(parts) == 2 && r.Method == http.MethodGet {
		s.handleJSONRelay(w, r, route, deviceId, "workspaces", nil, "")
		return
	}
	if len(parts) == 3 && parts[2] == "tree" && r.Method == http.MethodGet {
		s.handleJSONRelay(w, r, route, deviceId, "workspace_tree", nil, "workspace_tree")
		return
	}
	if len(parts) == 3 && parts[2] == "order" && r.Method == http.MethodPatch {
		var request struct {
			WorkspaceIds []string `json:"workspace_ids"`
		}
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		s.handleJSONRelay(w, r, route, deviceId, "workspace_order", request, "")
		return
	}
	if len(parts) == 3 && r.Method == http.MethodDelete {
		s.handleJSONRelay(w, r, route, deviceId, "delete_workspace", map[string]string{"workspace_id": parts[2]}, "")
		return
	}
	if len(parts) >= 4 && parts[3] == "sessions" {
		s.handleWorkspaceSessionRoute(w, r, route, deviceId, parts[2], parts[4:])
		return
	}
	s.writeNotFound(w, r)
}

func (s *Handler) handleWorkspaceSessionRoute(w http.ResponseWriter, r *http.Request, route *agentRoute, deviceId string, workspaceId string, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			s.handleJSONRelay(w, r, route, deviceId, "workspace_sessions", map[string]string{"workspace_id": workspaceId}, "")
		case http.MethodPost:
			var request json.RawMessage
			if !s.decodeJSONRequest(w, r, &request) {
				return
			}
			s.handleJSONRelayWithStatus(w, r, route, deviceId, "create_session", workspaceSessionParams(workspaceId, request), "", http.StatusCreated)
		default:
			s.methodNotAllowed(w, r, http.MethodGet, http.MethodPost)
		}
		return
	}
	sessionId := parts[0]
	if sessionId == "" {
		s.writeNotFound(w, r)
		return
	}
	params := map[string]string{"workspace_id": workspaceId, "session_id": sessionId}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleJSONRelay(w, r, route, deviceId, "get_session", params, "")
		case http.MethodPatch:
			var request json.RawMessage
			if !s.decodeJSONRequest(w, r, &request) {
				return
			}
			s.handleJSONRelay(w, r, route, deviceId, "update_session", map[string]any{"workspace_id": workspaceId, "session_id": sessionId, "request": request}, "")
		case http.MethodDelete:
			s.handleNoContentRelay(w, r, route, "delete_session", params)
		default:
			s.methodNotAllowed(w, r, http.MethodGet, http.MethodPatch, http.MethodDelete)
		}
		return
	}
	if len(parts) != 2 {
		s.writeNotFound(w, r)
		return
	}
	switch parts[1] {
	case "close":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, r, http.MethodPost)
			return
		}
		s.handleJSONRelay(w, r, route, deviceId, "close_session", params, "")
	case "rerun":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, r, http.MethodPost)
			return
		}
		var request json.RawMessage
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		s.handleJSONRelay(w, r, route, deviceId, "rerun_session", map[string]any{"workspace_id": workspaceId, "session_id": sessionId, "request": request}, "")
	case "history":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		s.handleHistoryRelay(w, r, route, deviceId, workspaceId, sessionId)
	case "ws":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		if route == nil {
			s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeDeviceOffline, errorMessageDeviceOffline, nil)
			return
		}
		s.handleTerminalWS(w, r, route, workspaceId, sessionId)
	default:
		s.writeNotFound(w, r)
	}
}

func workspaceSessionParams(workspaceId string, request json.RawMessage) map[string]any {
	var params map[string]any
	if err := json.Unmarshal(request, &params); err != nil || params == nil {
		params = map[string]any{}
	}
	params["workspace_id"] = workspaceId
	return params
}

func (s *Handler) handleJSONRelay(w http.ResponseWriter, r *http.Request, route *agentRoute, deviceId string, method string, params any, cacheKind string) {
	s.handleJSONRelayWithStatus(w, r, route, deviceId, method, params, cacheKind, http.StatusOK)
}

func (s *Handler) handleJSONRelayWithStatus(w http.ResponseWriter, r *http.Request, route *agentRoute, deviceId string, method string, params any, cacheKind string, status int) {
	if route == nil {
		if cached, ok := s.cachedJSON(deviceId, cacheKind); ok {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-TermBridge-Offline", "true")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeDeviceOffline, errorMessageDeviceOffline, nil)
		return
	}
	result, err := route.request(r.Context(), method, params, s.requestIdFor(w, r))
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	if cacheKind != "" {
		s.storeJSONCache(deviceId, cacheKind, result)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(result)
}

func (s *Handler) handleNoContentRelay(w http.ResponseWriter, r *http.Request, route *agentRoute, method string, params any) {
	if route == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeDeviceOffline, errorMessageDeviceOffline, nil)
		return
	}
	if _, err := route.request(r.Context(), method, params, s.requestIdFor(w, r)); err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Handler) handleHistoryRelay(w http.ResponseWriter, r *http.Request, route *agentRoute, deviceId string, workspaceId string, sessionId string) {
	if route == nil {
		if cached, ok := s.cachedHistory(deviceId, workspaceId, sessionId); ok {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-TermBridge-Offline", "true")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(cached))
			return
		}
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeDeviceOffline, errorMessageDeviceOffline, nil)
		return
	}
	result, err := route.request(r.Context(), "history", map[string]string{"workspace_id": workspaceId, "session_id": sessionId}, s.requestIdFor(w, r))
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	var text string
	if err := json.Unmarshal(result, &text); err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstreamInvalid, err)
		return
	}
	s.storeHistoryCache(deviceId, workspaceId, sessionId, text)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

func (s *Handler) handleTerminalWS(w http.ResponseWriter, r *http.Request, route *agentRoute, workspaceId string, sessionId string) {
	writerKey := sessionScopeKey(workspaceId, sessionId)
	s.writerMu.Lock()
	if _, exists := s.writers[writerKey]; exists {
		s.writerMu.Unlock()
		s.writeAPIError(w, r, http.StatusConflict, errorCodeConflict, errorMessageConflict, nil)
		return
	}
	streamId := tunnel.StreamId("term-" + strconv.FormatInt(time.Now().UnixNano(), 10))
	s.writers[writerKey] = streamId
	s.writerMu.Unlock()
	defer func() {
		s.writerMu.Lock()
		delete(s.writers, writerKey)
		s.writerMu.Unlock()
		route.removeTerminal(streamId)
	}()
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{terminalproto.Subprotocol}, OriginPatterns: s.originPatterns(r)})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	term := &terminalRelay{sessionId: sessionId, browser: conn, done: make(chan struct{})}
	route.addTerminal(streamId, term)
	attach, err := tunnel.NewFrame(streamId, tunnel.FrameTerminalAttach, tunnel.TerminalAttachReq{WorkspaceId: workspaceId, SessionId: sessionId, RequestId: s.requestIdFor(w, r)})
	if err != nil || route.writeFrame(r.Context(), attach) != nil {
		return
	}
	go func() {
		for {
			messageType, data, err := conn.Read(r.Context())
			if err != nil {
				closeFrame, _ := tunnel.NewFrame(streamId, tunnel.FrameClose, nil)
				_ = route.writeFrame(context.Background(), closeFrame)
				closeOnce(term.done)
				return
			}
			switch messageType {
			case websocket.MessageText:
				message, err := terminalproto.DecodeClient(data)
				if err != nil {
					_ = writeTerminalControl(conn, terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: err.Error()})
					continue
				}
				switch message.Type {
				case terminalproto.TypeHello:
					continue
				case terminalproto.TypeResize:
					resize, _ := tunnel.NewFrame(streamId, tunnel.FrameTerminalResize, tunnel.TerminalResizePayload{Cols: message.Cols, Rows: message.Rows})
					_ = route.writeFrame(r.Context(), resize)
				case terminalproto.TypeDetach:
					closeFrame, _ := tunnel.NewFrame(streamId, tunnel.FrameClose, nil)
					_ = route.writeFrame(context.Background(), closeFrame)
					closeOnce(term.done)
					return
				case terminalproto.TypePing:
					_ = writeTerminalControl(conn, terminalproto.ServerMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
				}
			case websocket.MessageBinary:
				input, _ := tunnel.NewFrame(streamId, tunnel.FrameTerminalInput, tunnel.TerminalDataPayload{Data: data})
				_ = route.writeFrame(r.Context(), input)
			}
		}
	}()
	<-term.done
}

func writeTerminalControl(conn *websocket.Conn, message terminalproto.ServerMessage) error {
	data, err := terminalproto.EncodeServer(message)
	if err != nil {
		return err
	}
	return conn.Write(context.Background(), websocket.MessageText, data)
}

func (s *Handler) handleAgentTunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	username, password, ok := r.BasicAuth()
	if !ok || !s.auth.ValidCredentials(username, password) {
		w.Header().Set("WWW-Authenticate", `Basic realm="termbridge-gateway"`)
		s.writeUnauthorized(w, r)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(tunnel.MaxFrameBytes)
	defer conn.Close(websocket.StatusNormalClosure, "")
	_, data, err := conn.Read(r.Context())
	if err != nil {
		return
	}
	frame, err := tunnel.Decode(data)
	if err != nil || frame.Type != tunnel.FrameHello {
		_ = conn.Close(websocket.StatusPolicyViolation, "expected hello")
		return
	}
	hello, err := tunnel.DecodePayload[tunnel.HelloPayload](frame)
	if err != nil || hello.DeviceId == "" || hello.DeviceName == "" {
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid hello")
		return
	}
	s.registry.Register(hello.DeviceId, hello.DeviceName, time.Now().UTC())
	route := newAgentRoute(hello.DeviceId, conn)
	s.setRoute(hello.DeviceId, route)
	defer func() {
		s.clearRoute(hello.DeviceId, route)
		route.closeTerminals("device disconnected")
		s.registry.MarkOffline(hello.DeviceId, time.Now().UTC())
	}()
	ack, err := tunnel.NewFrame(tunnel.ControlStreamId, tunnel.FrameHelloAck, tunnel.HelloAckPayload{ProtocolVersion: tunnel.ProtocolVersion})
	if err != nil {
		return
	}
	if err := route.writeFrame(r.Context(), ack); err != nil {
		return
	}
	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		frame, err := tunnel.Decode(data)
		if err != nil {
			continue
		}
		if route.dispatch(frame) {
			continue
		}
		if frame.Type == tunnel.FramePing {
			pong, err := tunnel.NewFrame(tunnel.ControlStreamId, tunnel.FramePong, nil)
			if err != nil {
				continue
			}
			_ = route.writeFrame(r.Context(), pong)
		}
	}
}

func (s *Handler) setRoute(deviceId string, route *agentRoute) {
	s.routeMu.Lock()
	old := s.routes[deviceId]
	s.routes[deviceId] = route
	s.routeMu.Unlock()
	if old != nil {
		old.closeTerminals("device reconnected")
		_ = old.conn.Close(websocket.StatusGoingAway, "device reconnected")
	}
}

func (s *Handler) clearRoute(deviceId string, route *agentRoute) {
	s.routeMu.Lock()
	if s.routes[deviceId] == route {
		delete(s.routes, deviceId)
	}
	s.routeMu.Unlock()
}

func (s *Handler) routeFor(deviceId string) *agentRoute {
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	return s.routes[deviceId]
}

func (s *Handler) cachedJSON(deviceId string, kind string) (json.RawMessage, bool) {
	if kind == "" {
		return nil, false
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	switch kind {
	case "workspace_tree":
		value, ok := s.workspaceTreeCache[deviceId]
		return append(json.RawMessage(nil), value...), ok
	default:
		return nil, false
	}
}

func (s *Handler) storeJSONCache(deviceId string, kind string, value json.RawMessage) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	switch kind {
	case "workspace_tree":
		s.workspaceTreeCache[deviceId] = append(json.RawMessage(nil), value...)
	}
}

func (s *Handler) cachedHistory(deviceId string, workspaceId string, sessionId string) (string, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	bySession := s.historyCache[deviceId]
	if bySession == nil {
		return "", false
	}
	value, ok := bySession[sessionScopeKey(workspaceId, sessionId)]
	return value, ok
}

func (s *Handler) storeHistoryCache(deviceId string, workspaceId string, sessionId string, value string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	bySession := s.historyCache[deviceId]
	if bySession == nil {
		bySession = map[string]string{}
		s.historyCache[deviceId] = bySession
	}
	bySession[sessionScopeKey(workspaceId, sessionId)] = value
}

func sessionScopeKey(workspaceId string, sessionId string) string {
	return workspaceId + "/" + sessionId
}

func (s *Handler) originPatterns(r *http.Request) []string {
	patterns := []string{"http://" + r.Host, "https://" + r.Host}
	patterns = append(patterns, s.config.AllowedOrigins...)
	return patterns
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func normalizeConfig(config Config) Config {
	if config.Logger == nil {
		panic("gateway api logger is required")
	}
	return config
}
