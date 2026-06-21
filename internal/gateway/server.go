package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	apperrors "termbridge-go/internal/errors"
	"termbridge-go/internal/gatewayauth"
	"termbridge-go/internal/terminalproto"
	"termbridge-go/internal/tunnel"
)

type Config struct {
	Host   string
	Port   int
	Open   bool
	Dev    bool
	Logger *slog.Logger
}

type Server struct {
	config   Config
	auth     *gatewayauth.Manager
	registry *DeviceRegistry
	routes   map[string]*agentRoute
	routeMu  sync.Mutex
	writers  map[string]tunnel.StreamID
	writerMu sync.Mutex
	server   *http.Server
}

type DeviceSummary struct {
	ID          string    `json:"id"`
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
	device := DeviceSummary{ID: id, Name: name, Online: true, ConnectedAt: now, LastSeen: now}
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

type Info struct {
	URL string
}

func New(config Config) *Server {
	config = normalizeConfig(config)
	mux := http.NewServeMux()
	s := &Server{config: config, auth: gatewayauth.NewManager(), registry: NewDeviceRegistry(), routes: map[string]*agentRoute{}, writers: map[string]tunnel.StreamID{}}
	mux.HandleFunc("/api/gateway/health", s.handleHealth)
	mux.HandleFunc("/api/gateway/login", s.handleLogin)
	mux.HandleFunc("/api/gateway/logout", s.auth.Middleware(http.HandlerFunc(s.handleLogout)).ServeHTTP)
	mux.HandleFunc("/api/gateway/me", s.auth.Middleware(http.HandlerFunc(s.handleMe)).ServeHTTP)
	mux.HandleFunc("/api/gateway/devices", s.auth.Middleware(http.HandlerFunc(s.handleDevices)).ServeHTTP)
	mux.HandleFunc("/api/gateway/devices/", s.auth.Middleware(http.HandlerFunc(s.handleDevice)).ServeHTTP)
	mux.HandleFunc("/api/gateway/agent/tunnel", s.handleAgentTunnel)
	s.server = &http.Server{Handler: mux}
	return s
}

func (s *Server) Listen() (net.Listener, Info, error) {
	addr := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, Info{}, apperrors.Runtime("listen gateway server", err)
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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !s.auth.Login(w, request.Username, request.Password) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	s.auth.Logout(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": gatewayauth.Username})
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, s.registry.List())
}

func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/gateway/devices/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	deviceID := parts[0]
	route := s.routeFor(deviceID)
	if route == nil {
		http.Error(w, "route unavailable", http.StatusServiceUnavailable)
		return
	}
	if len(parts) == 2 && parts[1] == "sessions" && r.Method == http.MethodGet {
		s.handleRelayRequest(w, r, route, "sessions", nil)
		return
	}
	if len(parts) == 3 && parts[1] == "workspaces" && parts[2] == "tree" && r.Method == http.MethodGet {
		s.handleRelayRequest(w, r, route, "workspace_tree", nil)
		return
	}
	if len(parts) == 4 && parts[1] == "sessions" && parts[3] == "history" && r.Method == http.MethodGet {
		s.handleHistoryRelay(w, r, route, parts[2])
		return
	}
	if len(parts) == 4 && parts[1] == "sessions" && parts[3] == "ws" && r.Method == http.MethodGet {
		s.handleTerminalWS(w, r, route, parts[2])
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleRelayRequest(w http.ResponseWriter, r *http.Request, route *agentRoute, method string, params any) {
	result, err := route.request(r.Context(), method, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result)
}

func (s *Server) handleHistoryRelay(w http.ResponseWriter, r *http.Request, route *agentRoute, sessionID string) {
	result, err := route.request(r.Context(), "history", map[string]string{"session_id": sessionID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	var text string
	if err := json.Unmarshal(result, &text); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request, route *agentRoute, sessionID string) {
	s.writerMu.Lock()
	if _, exists := s.writers[sessionID]; exists {
		s.writerMu.Unlock()
		http.Error(w, "session already has an active writer", http.StatusConflict)
		return
	}
	streamID := tunnel.StreamID("term-" + strconv.FormatInt(time.Now().UnixNano(), 10))
	s.writers[sessionID] = streamID
	s.writerMu.Unlock()
	defer func() {
		s.writerMu.Lock()
		delete(s.writers, sessionID)
		s.writerMu.Unlock()
		route.removeTerminal(streamID)
	}()
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{terminalproto.Subprotocol}})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	term := &terminalRelay{sessionID: sessionID, browser: conn, done: make(chan struct{})}
	route.addTerminal(streamID, term)
	attach, err := tunnel.NewFrame(streamID, tunnel.FrameTerminalAttach, tunnel.TerminalAttachPayload{SessionID: sessionID})
	if err != nil || route.writeFrame(r.Context(), attach) != nil {
		return
	}
	go func() {
		for {
			messageType, data, err := conn.Read(r.Context())
			if err != nil {
				closeFrame, _ := tunnel.NewFrame(streamID, tunnel.FrameClose, nil)
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
					resize, _ := tunnel.NewFrame(streamID, tunnel.FrameTerminalResize, tunnel.TerminalResizePayload{Cols: message.Cols, Rows: message.Rows})
					_ = route.writeFrame(r.Context(), resize)
				case terminalproto.TypeDetach:
					closeFrame, _ := tunnel.NewFrame(streamID, tunnel.FrameClose, nil)
					_ = route.writeFrame(context.Background(), closeFrame)
					closeOnce(term.done)
					return
				case terminalproto.TypePing:
					_ = writeTerminalControl(conn, terminalproto.ServerMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
				}
			case websocket.MessageBinary:
				input, _ := tunnel.NewFrame(streamID, tunnel.FrameTerminalInput, tunnel.TerminalDataPayload{Data: string(data)})
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

func (s *Server) handleAgentTunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	username, password, ok := r.BasicAuth()
	if !ok || !gatewayauth.ValidCredentials(username, password) {
		w.Header().Set("WWW-Authenticate", `Basic realm="termbridge-gateway"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
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
	if err != nil || hello.DeviceID == "" || hello.DeviceName == "" {
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid hello")
		return
	}
	s.registry.Register(hello.DeviceID, hello.DeviceName, time.Now().UTC())
	route := newAgentRoute(hello.DeviceID, conn)
	s.setRoute(hello.DeviceID, route)
	defer func() {
		s.clearRoute(hello.DeviceID, route)
		route.closeTerminals("device disconnected")
		s.registry.MarkOffline(hello.DeviceID, time.Now().UTC())
	}()
	ack, err := tunnel.NewFrame(tunnel.ControlStreamID, tunnel.FrameHelloAck, tunnel.HelloAckPayload{ProtocolVersion: tunnel.ProtocolVersion})
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
			pong, err := tunnel.NewFrame(tunnel.ControlStreamID, tunnel.FramePong, nil)
			if err != nil {
				continue
			}
			_ = route.writeFrame(r.Context(), pong)
		}
	}
}

func (s *Server) setRoute(deviceID string, route *agentRoute) {
	s.routeMu.Lock()
	old := s.routes[deviceID]
	s.routes[deviceID] = route
	s.routeMu.Unlock()
	if old != nil {
		old.closeTerminals("device reconnected")
		_ = old.conn.Close(websocket.StatusGoingAway, "device reconnected")
	}
}

func (s *Server) clearRoute(deviceID string, route *agentRoute) {
	s.routeMu.Lock()
	if s.routes[deviceID] == route {
		delete(s.routes, deviceID)
	}
	s.routeMu.Unlock()
}

func (s *Server) routeFor(deviceID string) *agentRoute {
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	return s.routes[deviceID]
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func normalizeConfig(config Config) Config {
	if config.Host == "" {
		config.Host = "127.0.0.1"
	}
	if config.Port == 0 {
		config.Port = 8080
	}
	return config
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func openBrowser(url string) {
	commands := [][]string{
		{"open", url},
		{"xdg-open", url},
		{"rundll32", "url.dll,FileProtocolHandler", url},
	}
	for _, command := range commands {
		if _, err := exec.LookPath(command[0]); err != nil {
			continue
		}
		_ = exec.Command(command[0], command[1:]...).Start()
		return
	}
}
