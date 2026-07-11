package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"

	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	cloudproto "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	sharedAuth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/prototime"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
)

type Config struct {
	JWTSecret           []byte
	DebugErrors         bool
	Logger              *slog.Logger
	AuthService         AuthService
	CloudPublicURL      string
	OAuthClient         OAuthClientConfig
	LocalDevice         agentapp.Device
	LocalDeviceStateDir string
	LocalRuntime        agentapp.RuntimeAccess
	CORSAllowedOrigins  []string
	OnLocalCloudSession func(*cloudproto.CloudSessionSummary) // shared auth via sharedAuth
}

type OAuthClientConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
	Scopes       []string
}

type AuthService interface {
	Login(ctx context.Context, email, password string) (*cloudproto.LocalAuthLoginResp, error)
	UserFromClaims(ctx context.Context, claims sharedAuth.Claims) (*cloudproto.User, error)
	VerifyToken(token string) (sharedAuth.Claims, error)
}

type Handler struct {
	config             Config
	auth               *sharedAuth.Auther
	authService        AuthService
	writers            map[string]string
	writerMu           sync.Mutex
	cacheMu            sync.Mutex
	workspaceTreeCache map[string]json.RawMessage
	historyCache       map[string]map[string]string
	cloudSessionMu     sync.Mutex
	cloudSession       *cloudproto.CloudSessionSummary
	localRuntime       runtimeEndpoint
}

func New(config Config) *Handler {
	config = normalizeConfig(config)
	handler := &Handler{config: config, auth: sharedAuth.NewAuther(sharedAuth.Credentials{}, sharedAuth.NewTokenService(config.JWTSecret)), authService: config.AuthService, writers: map[string]string{}, workspaceTreeCache: map[string]json.RawMessage{}, historyCache: map[string]map[string]string{}}
	if config.LocalRuntime != nil {
		handler.localRuntime = localRuntimeEndpoint{runtime: config.LocalRuntime, handler: handler}
	}
	return handler
}

func (h *Handler) Handler() http.Handler {
	mux := http.NewServeMux()
	h.registerCommonRoutes(mux)
	h.registerAgentRoutes(mux)
	mux.HandleFunc("/local-api", h.writeNotFound)
	mux.HandleFunc("/local-api/", h.writeNotFound)
	return mux
}

func (h *Handler) registerCommonRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/local-api/health", h.handleHealth)
	mux.HandleFunc("/local-api/auth/login", h.handleAuthLogin)
	mux.HandleFunc("/local-api/auth/logout", h.authMiddleware(http.HandlerFunc(h.handleLogout)).ServeHTTP)
	mux.HandleFunc("/local-api/auth/me", h.handleAuthMe)
}

func (h *Handler) registerAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/local-api/cloud/connect", h.authMiddleware(http.HandlerFunc(h.handleCloudConnect)).ServeHTTP)
	mux.HandleFunc("/local-api/cloud/disconnect", h.authMiddleware(http.HandlerFunc(h.handleCloudDisconnect)).ServeHTTP)
	mux.HandleFunc("/local-api/cloud/oauth/exchange", h.authMiddleware(http.HandlerFunc(h.handleExchangeOAuthCode)).ServeHTTP)
	mux.HandleFunc("/local-api/workspaces", h.authMiddleware(http.HandlerFunc(h.handleLocalWorkspaces)).ServeHTTP)
	mux.HandleFunc("/local-api/workspaces/", h.authMiddleware(http.HandlerFunc(h.handleLocalWorkspaces)).ServeHTTP)
	mux.HandleFunc("/local-api/sessions", h.authMiddleware(http.HandlerFunc(h.handleLocalSessions)).ServeHTTP)
	mux.HandleFunc("/local-api/shortcuts", h.authMiddleware(http.HandlerFunc(h.handleLocalShortcuts)).ServeHTTP)
	mux.HandleFunc("/local-api/shortcuts/", h.authMiddleware(http.HandlerFunc(h.handleLocalShortcuts)).ServeHTTP)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.Handler().ServeHTTP(w, r) }

func (s *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, &shared.HealthResp{Status: "ok"})
}

func (s *Handler) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if s.authService != nil {
		result, err := s.authService.Login(r.Context(), "", "")
		if err != nil {
			s.writeUnauthorized(w, r)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	if s.auth == nil {
		s.writeUnauthorized(w, r)
		return
	}
	token, err := s.auth.SignToken("local-agent")
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, "Failed to sign token", err)
		return
	}
	writeJSON(w, http.StatusOK, &cloudproto.LocalAuthLoginResp{AccessToken: token, TokenType: "bearer"})
}

func (s *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Handler) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	if s.authService != nil {
		cloudSession := s.localCloudSessionSummary()
		device := s.localDeviceSummary()
		claims, ok := s.claimsFromRequest(r)
		if !ok {
			writeJSON(w, http.StatusOK, &cloudproto.AuthMeResp{Authenticated: false, CloudSession: cloudSession, Device: device})
			return
		}
		user, err := s.authService.UserFromClaims(r.Context(), claims)
		if err != nil {
			s.writeUnauthorized(w, r)
			return
		}
		writeJSON(w, http.StatusOK, &cloudproto.AuthMeResp{Authenticated: true, User: user, CloudSession: cloudSession, Device: device})
		return
	}
	writeJSON(w, http.StatusOK, &cloudproto.AuthMeResp{Authenticated: true, Username: s.auth.UsernameFromRequest(r), Device: s.localDeviceSummary()})
}

func (s *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.authService != nil {
			if _, ok := s.claimsFromRequest(r); !ok {
				s.writeUnauthorized(w, r)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		s.auth.Middleware(next, s.writeUnauthorized).ServeHTTP(w, r)
	})
}
func (s *Handler) localCloudSessionSummary() *cloudproto.CloudSessionSummary {
	s.cloudSessionMu.Lock()
	defer s.cloudSessionMu.Unlock()
	if s.cloudSession == nil {
		return nil
	}
	return proto.Clone(s.cloudSession).(*cloudproto.CloudSessionSummary)
}

func (s *Handler) localDeviceSummary() *cloudproto.DeviceSummary {
	device := s.config.LocalDevice
	if strings.TrimSpace(device.Id) == "" && strings.TrimSpace(device.Name) == "" {
		return nil
	}
	return &cloudproto.DeviceSummary{Id: device.Id, Name: device.Name, Online: true, Status: "online"}
}

func (s *Handler) setLocalCloudSession(summary *cloudproto.CloudSessionSummary) {
	s.cloudSessionMu.Lock()
	defer s.cloudSessionMu.Unlock()
	if summary == nil {
		s.cloudSession = nil
		return
	}
	s.cloudSession = proto.Clone(summary).(*cloudproto.CloudSessionSummary)
}

func (s *Handler) claimsFromRequest(r *http.Request) (sharedAuth.Claims, bool) {
	token := sharedAuth.ExtractBearerToken(r)
	if token == "" || s.authService == nil {
		return sharedAuth.Claims{}, false
	}
	claims, err := s.authService.VerifyToken(token)
	return claims, err == nil
}
func (s *Handler) handleCloudConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if strings.TrimSpace(s.config.CloudPublicURL) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Cloud public URL is not configured.", nil)
		return
	}
	var req cloudproto.CloudConnectReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	cloudToken := strings.TrimSpace(req.GetCloudToken())
	if cloudToken == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Cloud token is required.", nil)
		return
	}
	summary, err := s.connectCloudWithToken(r.Context(), cloudToken)
	if err != nil {
		s.config.Logger.Warn("cloud connect failed", "error", err)
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	s.persistLocalCloudSession(summary)
	writeJSON(w, http.StatusOK, &cloudproto.CloudConnectResp{CloudSession: summary})
}

func (s *Handler) persistLocalCloudSession(summary *cloudproto.CloudSessionSummary) {
	s.setLocalCloudSession(summary)
	if s.config.OnLocalCloudSession != nil {
		s.config.OnLocalCloudSession(summary)
	}
}

func (s *Handler) handleCloudDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	s.setLocalCloudSession(nil)
	if s.config.OnLocalCloudSession != nil {
		s.config.OnLocalCloudSession(nil)
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Handler) handleLocalWorkspaces(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/local-api/workspaces"), "/")
	parts := []string{"", "workspaces"}
	if path != "" {
		parts = append(parts, strings.Split(path, "/")...)
	}
	s.handleWorkspaceRoute(w, r, s.localRuntime, "", parts)
}

func (s *Handler) handleLocalSessions(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/local-api/sessions" {
		s.writeNotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	request := &agent.CreateSessionReq{}
	if !s.decodeJSONRequest(w, r, request) {
		return
	}
	s.handleJSONRuntimeWithStatus(w, r, s.localRuntime, "", "create_session", request, "", http.StatusCreated)
}

func (s *Handler) handleLocalShortcuts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/local-api/shortcuts")
	if path == "" {
		s.handleShortcutRoute(w, r, s.localRuntime, "", nil)
		return
	}
	if !strings.HasPrefix(path, "/") {
		s.writeNotFound(w, r)
		return
	}
	shortcutId := strings.TrimPrefix(path, "/")
	if shortcutId == "" || strings.Contains(shortcutId, "/") {
		s.writeNotFound(w, r)
		return
	}
	s.handleShortcutRoute(w, r, s.localRuntime, "", &shortcutId)
}

func (s *Handler) handleShortcutRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, shortcutId *string) {
	if shortcutId == nil {
		switch r.Method {
		case http.MethodGet:
			s.handleJSONRuntime(w, r, endpoint, deviceId, "list_shortcuts", nil, "")
		case http.MethodPost:
			request := &agent.CreateShortcutReq{}
			if !s.decodeJSONRequest(w, r, request) {
				return
			}
			s.handleJSONRuntimeWithStatus(w, r, endpoint, deviceId, "create_shortcut", request, "", http.StatusCreated)
		default:
			s.methodNotAllowed(w, r, http.MethodGet, http.MethodPost)
		}
		return
	}
	switch r.Method {
	case http.MethodPatch:
		request := &agent.UpdateShortcutReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "update_shortcut", &agent.UpdateShortcutRequest{ShortcutId: *shortcutId, Request: request}, "")
	case http.MethodDelete:
		s.handleNoContentRuntime(w, r, endpoint, "delete_shortcut", &agent.DeleteShortcutReq{ShortcutId: *shortcutId})
	default:
		s.methodNotAllowed(w, r, http.MethodPatch, http.MethodDelete)
	}
}

func (s *Handler) handleWorkspaceRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, parts []string) {
	if len(parts) == 2 && r.Method == http.MethodGet {
		s.handleJSONRuntime(w, r, endpoint, deviceId, "workspaces", nil, "")
		return
	}
	if len(parts) == 3 && parts[2] == "tree" && r.Method == http.MethodGet {
		s.handleJSONRuntime(w, r, endpoint, deviceId, "workspace_tree", nil, "workspace_tree")
		return
	}
	if len(parts) == 3 && parts[2] == "order" && r.Method == http.MethodPatch {
		request := &agent.UpdateWorkspaceOrderReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "workspace_order", request, "")
		return
	}
	if len(parts) == 3 && r.Method == http.MethodDelete {
		s.handleNoContentRuntime(w, r, endpoint, "delete_workspace", &agent.DeleteWorkspaceReq{WorkspaceId: parts[2]})
		return
	}
	if len(parts) >= 4 && parts[3] == "sessions" {
		s.handleWorkspaceSessionRoute(w, r, endpoint, deviceId, parts[2], parts[4:])
		return
	}
	s.writeNotFound(w, r)
}
func (s *Handler) handleWorkspaceSessionRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			s.handleJSONRuntime(w, r, endpoint, deviceId, "workspace_sessions", &agent.WorkspaceSessionsReq{WorkspaceId: workspaceId}, "")
		case http.MethodPost:
			request := &agent.CreateSessionReq{}
			if !s.decodeJSONRequest(w, r, request) {
				return
			}
			request.WorkspaceId = workspaceId
			s.handleJSONRuntimeWithStatus(w, r, endpoint, deviceId, "create_session", request, "", http.StatusCreated)
		default:
			s.methodNotAllowed(w, r, http.MethodGet, http.MethodPost)
		}
		return
	}
	if len(parts) == 1 && parts[0] == "order" && r.Method == http.MethodPatch {
		request := &agent.UpdateSessionOrderReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "session_order", &agent.WorkspaceSessionOrderReq{WorkspaceId: workspaceId, SessionIds: request.GetSessionIds()}, "")
		return
	}
	sessionId := parts[0]
	if sessionId == "" {
		s.writeNotFound(w, r)
		return
	}
	params := &agent.WorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleJSONRuntime(w, r, endpoint, deviceId, "get_session", params, "")
		case http.MethodPatch:
			request := &agent.UpdateSessionReq{}
			if !s.decodeJSONRequest(w, r, request) {
				return
			}
			s.handleJSONRuntime(w, r, endpoint, deviceId, "update_session", &agent.UpdateWorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId, Request: request}, "")
		case http.MethodDelete:
			s.handleNoContentRuntime(w, r, endpoint, "delete_session", params)
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
		s.handleJSONRuntime(w, r, endpoint, deviceId, "close_session", params, "")
	case "rerun":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, r, http.MethodPost)
			return
		}
		request := &agent.RerunSessionReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "rerun_session", &agent.RerunWorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId, Request: request}, "")
	case "history":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		s.handleHistoryRuntime(w, r, endpoint, deviceId, workspaceId, sessionId)
	case "ws":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, r, http.MethodGet)
			return
		}
		if !runtimeEndpointAvailable(endpoint) {
			s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeDeviceOffline, errorMessageDeviceOffline, nil)
			return
		}
		if err := endpoint.Attach(w, r, workspaceId, sessionId); err != nil {
			s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		}
	default:
		s.writeNotFound(w, r)
	}
}
func runtimeEndpointAvailable(endpoint runtimeEndpoint) bool {
	return endpoint != nil && endpoint.Available()
}

func (s *Handler) handleJSONRuntime(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, method string, params any, cacheKind string) {
	s.handleJSONRuntimeWithStatus(w, r, endpoint, deviceId, method, params, cacheKind, http.StatusOK)
}
func (s *Handler) handleJSONRuntimeWithStatus(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, method string, params any, cacheKind string, status int) {
	if !runtimeEndpointAvailable(endpoint) {
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
	result, err := endpoint.JSON(r.Context(), method, params, s.requestIdFor(w, r))
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	if cacheKind != "" && deviceId != "" {
		s.storeJSONCache(deviceId, cacheKind, result)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(result)
}
func (s *Handler) handleNoContentRuntime(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, method string, params any) {
	if !runtimeEndpointAvailable(endpoint) {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeDeviceOffline, errorMessageDeviceOffline, nil)
		return
	}
	if _, err := endpoint.JSON(r.Context(), method, params, s.requestIdFor(w, r)); err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Handler) handleHistoryRuntime(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, workspaceId string, sessionId string) {
	if !runtimeEndpointAvailable(endpoint) {
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
	text, offline, err := endpoint.History(r.Context(), workspaceId, sessionId, s.requestIdFor(w, r))
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	if deviceId != "" {
		s.storeHistoryCache(deviceId, workspaceId, sessionId, text)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if offline {
		w.Header().Set("X-TermBridge-Offline", "true")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

func terminalAttachSizeFromQuery(r *http.Request) (int, int, bool, error) {
	colsRaw := strings.TrimSpace(r.URL.Query().Get("cols"))
	rowsRaw := strings.TrimSpace(r.URL.Query().Get("rows"))
	if colsRaw == "" && rowsRaw == "" {
		return 0, 0, false, nil
	}
	if colsRaw == "" || rowsRaw == "" {
		return 0, 0, false, fmt.Errorf("terminal attach size requires both cols and rows")
	}
	cols, err := strconv.Atoi(colsRaw)
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid terminal attach cols: %w", err)
	}
	rows, err := strconv.Atoi(rowsRaw)
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid terminal attach rows: %w", err)
	}
	if err := terminalproto.ValidateSize(cols, rows); err != nil {
		return 0, 0, false, err
	}
	return cols, rows, true, nil
}
func writeTerminalControl(conn *websocket.Conn, message *agent.ServerControlMessage) error {
	data, err := terminalproto.EncodeServer(message)
	if err != nil {
		return err
	}
	return conn.Write(context.Background(), websocket.MessageText, data)
}

func (s *Handler) handleExchangeOAuthCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if strings.TrimSpace(s.config.CloudPublicURL) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Cloud public URL is not configured.", nil)
		return
	}
	if s.config.OAuthClient.ClientId == "" || s.config.OAuthClient.ClientSecret == "" || s.config.OAuthClient.RedirectUrl == "" {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeServiceUnavailable, "Cloud OAuth is not configured.", nil)
		return
	}
	var req cloudproto.CloudOAuthExchangeReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	code := strings.TrimSpace(req.GetCode())
	if code == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Authorization code is required.", nil)
		return
	}
	accessToken, err := s.exchangeCloudOAuthCode(r.Context(), code)
	if err != nil {
		s.config.Logger.Warn("cloud oauth token exchange failed", "error", err)
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	writeJSON(w, http.StatusOK, &cloudproto.CloudOAuthExchangeResp{AccessToken: accessToken, TokenType: "bearer"})
}

func (s *Handler) exchangeCloudOAuthCode(ctx context.Context, code string) (string, error) {
	cfg := oauth2.Config{
		ClientID:     s.config.OAuthClient.ClientId,
		ClientSecret: s.config.OAuthClient.ClientSecret,
		RedirectURL:  s.config.OAuthClient.RedirectUrl,
		Scopes:       append([]string(nil), s.config.OAuthClient.Scopes...),
		Endpoint: oauth2.Endpoint{
			TokenURL:  s.config.CloudPublicURL + "/cloud-api/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return "", fmt.Errorf("cloud oauth token response missing access token")
	}
	return token.AccessToken, nil
}

func (s *Handler) connectCloudWithToken(ctx context.Context, cloudToken string) (*cloudproto.CloudSessionSummary, error) {
	device := s.config.LocalDevice
	if device.Id == "" || device.Name == "" {
		return nil, fmt.Errorf("local device identity is incomplete")
	}
	if strings.TrimSpace(device.PublicKey) == "" {
		return nil, fmt.Errorf("local device public key is incomplete")
	}
	reportBody, err := codec.MarshalProtoJSON(&cloudproto.CurrentDeviceReq{Id: device.Id, Name: device.Name, PublicKey: device.PublicKey})
	if err != nil {
		return nil, err
	}
	reportReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.CloudPublicURL+"/cloud-api/devices/current", bytes.NewReader(reportBody))
	if err != nil {
		return nil, err
	}
	reportReq.Header.Set("Content-Type", "application/json")
	reportReq.Header.Set("Authorization", "Bearer "+cloudToken)
	reportResp, err := http.DefaultClient.Do(reportReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reportResp.Body.Close() }()
	if reportResp.StatusCode < 200 || reportResp.StatusCode >= 300 {
		return nil, fmt.Errorf("cloud device report status %d", reportResp.StatusCode)
	}
	return &cloudproto.CloudSessionSummary{PublicUrl: s.config.CloudPublicURL, DeviceId: device.Id, DeviceName: device.Name, ConnectedAt: prototime.FromTime(time.Now().UTC())}, nil
}

func (s *Handler) cachedJSON(deviceId string, kind string) (json.RawMessage, bool) {
	if kind == "" {
		return nil, false
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	switch kind {
	case "workspace_tree":
		v, ok := s.workspaceTreeCache[deviceId]
		return append(json.RawMessage(nil), v...), ok
	default:
		return nil, false
	}
}
func (s *Handler) storeJSONCache(deviceId string, kind string, value json.RawMessage) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if kind == "workspace_tree" {
		s.workspaceTreeCache[deviceId] = append(json.RawMessage(nil), value...)
	}
}
func (s *Handler) cachedHistory(deviceId string, workspaceId string, sessionId string) (string, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	by := s.historyCache[deviceId]
	if by == nil {
		return "", false
	}
	v, ok := by[sessionScopeKey(workspaceId, sessionId)]
	return v, ok
}
func (s *Handler) storeHistoryCache(deviceId string, workspaceId string, sessionId string, value string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	by := s.historyCache[deviceId]
	if by == nil {
		by = map[string]string{}
		s.historyCache[deviceId] = by
	}
	by[sessionScopeKey(workspaceId, sessionId)] = value
}
func sessionScopeKey(workspaceId string, sessionId string) string {
	return workspaceId + "/" + sessionId
}
func (s *Handler) originPatterns(r *http.Request) []string {
	patterns := []string{"http://" + r.Host, "https://" + r.Host}
	patterns = append(patterns, s.config.CORSAllowedOrigins...)
	return patterns
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

func normalizeConfig(config Config) Config {
	if config.Logger == nil {
		panic("agent api logger is required")
	}
	config.CloudPublicURL = strings.TrimRight(strings.TrimSpace(config.CloudPublicURL), "/")
	config.CORSAllowedOrigins = cleanOrigins(config.CORSAllowedOrigins)
	return config
}
