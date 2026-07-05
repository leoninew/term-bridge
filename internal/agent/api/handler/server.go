package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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

	terminalapp "termbridge-go/internal/agent/application/task/terminal"
	agentapp "termbridge-go/internal/agent/application/user"
	"termbridge-go/internal/shared/common/auth"
	terminalproto "termbridge-go/internal/shared/dto/protocol/terminal"
)

type Config struct {
	JWTSecret           []byte
	DebugErrors         bool
	Logger              *slog.Logger
	AuthService         AuthService
	CloudGateURL        string
	CloudOAuth          CloudOAuthConfig
	LocalDevice         agentapp.Device
	LocalDeviceStateDir string
	LocalRuntime        agentapp.RuntimeAccess
	CORSAllowedOrigins  []string
	OnLocalCloudSession func(CloudSessionSummary)
}

type CloudOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectUrl  string
	Scopes       []string
}

type AuthService interface {
	Login(ctx context.Context, email, password string) (AuthResult, error)
	UserFromClaims(ctx context.Context, claims auth.Claims) (UserView, error)
	VerifyToken(token string) (auth.Claims, error)
}

const cloudOAuthStateTtl = 10 * time.Minute

type CloudOAuthState struct {
	Value            string
	PostAuthRedirect string
	ExpiresAt        time.Time
}

type Handler struct {
	config              Config
	auth                *auth.Auther
	authService         AuthService
	writers             map[string]string
	writerMu            sync.Mutex
	cacheMu             sync.Mutex
	workspaceTreeCache  map[string]json.RawMessage
	historyCache        map[string]map[string]string
	cloudSessionMu      sync.Mutex
	cloudSession        *CloudSessionSummary
	cloudOAuthMu        sync.Mutex
	cloudOAuthStates    map[string]CloudOAuthState
	cloudOAuthConfigErr error
	localRuntime        runtimeEndpoint
}

func New(config Config) *Handler {
	config = normalizeConfig(config)
	handler := &Handler{config: config, auth: auth.NewAuther(auth.Credentials{}, auth.NewTokenService(config.JWTSecret)), authService: config.AuthService, writers: map[string]string{}, workspaceTreeCache: map[string]json.RawMessage{}, historyCache: map[string]map[string]string{}, cloudOAuthStates: map[string]CloudOAuthState{}, cloudOAuthConfigErr: validateCloudOAuthConfig(config)}
	if config.LocalDevice.CloudBinding != nil {
		handler.setLocalCloudSession(cloudSessionSummaryFromBinding(*config.LocalDevice.CloudBinding))
	} else if config.LocalDeviceStateDir != "" {
		if summary, err := agentapp.LoadCloudBindingSummary(config.LocalDeviceStateDir); err == nil && summary != nil {
			handler.setLocalCloudSession(cloudSessionSummaryFromBinding(*summary))
		} else if err != nil {
			config.Logger.Warn("load local cloud session", "error", err)
		}
	}
	if config.LocalRuntime != nil {
		handler.localRuntime = localRuntimeEndpoint{runtime: config.LocalRuntime, handler: handler}
	}
	return handler
}

func (h *Handler) Handler() http.Handler {
	mux := http.NewServeMux()
	h.registerCommonRoutes(mux)
	h.registerAgentRoutes(mux)
	mux.HandleFunc("/agent-api", h.writeNotFound)
	mux.HandleFunc("/agent-api/", h.writeNotFound)
	return mux
}

func (h *Handler) registerCommonRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/agent-api/health", h.handleHealth)
	mux.HandleFunc("/agent-api/auth/login", h.handleAuthLogin)
	mux.HandleFunc("/agent-api/auth/logout", h.authMiddleware(http.HandlerFunc(h.handleLogout)).ServeHTTP)
	mux.HandleFunc("/agent-api/auth/me", h.handleAuthMe)
}

func (h *Handler) registerAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/agent-api/cloud-oauth/start", h.handleCloudOAuthStart)
	mux.HandleFunc("/agent-api/cloud-oauth/callback", h.handleCloudOAuthCallback)
	mux.HandleFunc("/agent-api/workspaces", h.authMiddleware(http.HandlerFunc(h.handleLocalWorkspaces)).ServeHTTP)
	mux.HandleFunc("/agent-api/workspaces/", h.authMiddleware(http.HandlerFunc(h.handleLocalWorkspaces)).ServeHTTP)
	mux.HandleFunc("/agent-api/sessions", h.authMiddleware(http.HandlerFunc(h.handleLocalSessions)).ServeHTTP)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.Handler().ServeHTTP(w, r) }

func (s *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, HealthResp{Status: "ok"})
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
		writeJSON(w, http.StatusOK, TokenResp{AccessToken: result.Token, TokenType: "bearer"})
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
	writeJSON(w, http.StatusOK, TokenResp{AccessToken: token, TokenType: "bearer"})
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
		claims, ok := s.claimsFromRequest(r)
		if !ok {
			writeJSON(w, http.StatusOK, AuthMeResp{Authenticated: false, CloudSession: cloudSession})
			return
		}
		user, err := s.authService.UserFromClaims(r.Context(), claims)
		if err != nil {
			s.writeUnauthorized(w, r)
			return
		}
		writeJSON(w, http.StatusOK, AuthMeResp{Authenticated: true, User: &user, CloudSession: cloudSession})
		return
	}
	writeJSON(w, http.StatusOK, AuthMeResp{Authenticated: true, Username: s.auth.UsernameFromRequest(r)})
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
func (s *Handler) storeCloudOAuthState(state CloudOAuthState, now time.Time) {
	s.cloudOAuthMu.Lock()
	defer s.cloudOAuthMu.Unlock()
	for value, existing := range s.cloudOAuthStates {
		if !existing.ExpiresAt.IsZero() && !existing.ExpiresAt.After(now) {
			delete(s.cloudOAuthStates, value)
		}
	}
	s.cloudOAuthStates[state.Value] = state
}

func (s *Handler) consumeCloudOAuthState(value string, now time.Time) (CloudOAuthState, bool) {
	s.cloudOAuthMu.Lock()
	defer s.cloudOAuthMu.Unlock()
	state, ok := s.cloudOAuthStates[value]
	if ok {
		delete(s.cloudOAuthStates, value)
	}
	if !ok || state.ExpiresAt.IsZero() || !state.ExpiresAt.After(now) {
		return CloudOAuthState{}, false
	}
	return state, true
}

func randomCloudOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func validateCloudOAuthConfig(config Config) error {
	if config.CloudGateURL == "" {
		return fmt.Errorf("cloud gate URL is required")
	}
	if config.CloudOAuth.ClientID == "" {
		return fmt.Errorf("cloud OAuth client id is required")
	}
	if config.CloudOAuth.RedirectUrl == "" {
		return fmt.Errorf("cloud OAuth redirect URL is required")
	}
	return nil
}

func (s *Handler) localCloudSessionSummary() *CloudSessionSummary {
	s.cloudSessionMu.Lock()
	defer s.cloudSessionMu.Unlock()
	if s.cloudSession == nil {
		return nil
	}
	summary := *s.cloudSession
	return &summary
}

func (s *Handler) setLocalCloudSession(summary CloudSessionSummary) {
	s.cloudSessionMu.Lock()
	defer s.cloudSessionMu.Unlock()
	s.cloudSession = &summary
}

func cloudSessionSummaryFromBinding(summary agentapp.CloudBindingSummary) CloudSessionSummary {
	return CloudSessionSummary{GateURL: summary.GateUrl, DeviceId: summary.DeviceId, DeviceName: summary.DeviceName, ConnectedAt: summary.ConnectedAt}
}

func cloudBindingSummaryFromSession(summary CloudSessionSummary) agentapp.CloudBindingSummary {
	return agentapp.CloudBindingSummary{GateUrl: summary.GateURL, DeviceId: summary.DeviceId, DeviceName: summary.DeviceName, ConnectedAt: summary.ConnectedAt}
}

func (s *Handler) claimsFromRequest(r *http.Request) (auth.Claims, bool) {
	token := auth.ExtractBearerToken(r)
	if token == "" || s.authService == nil {
		return auth.Claims{}, false
	}
	claims, err := s.authService.VerifyToken(token)
	return claims, err == nil
}
func (s *Handler) handleCloudOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	if err := s.cloudOAuthConfigErr; err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Cloud OAuth is not configured.", err)
		return
	}
	state, err := randomCloudOAuthState()
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	now := time.Now().UTC()
	s.storeCloudOAuthState(CloudOAuthState{Value: state, PostAuthRedirect: r.URL.Query().Get("redirect"), ExpiresAt: now.Add(cloudOAuthStateTtl)}, now)
	oauthConfig := s.cloudOAuthConfig()
	writeJSON(w, http.StatusOK, CloudOAuthStartResp{AuthorizeURL: oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)})
}

func (s *Handler) cloudOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.config.CloudOAuth.ClientID,
		ClientSecret: s.config.CloudOAuth.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   s.config.CloudGateURL + "/oauth2/authorize",
			TokenURL:  s.config.CloudGateURL + "/cloud-api/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		RedirectURL: s.config.CloudOAuth.RedirectUrl,
		Scopes:      append([]string(nil), s.config.CloudOAuth.Scopes...),
	}
}

func (s *Handler) handleCloudOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if err := s.cloudOAuthConfigErr; err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Cloud OAuth is not configured.", err)
		return
	}
	var req CloudOAuthCallbackReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	stateValue := strings.TrimSpace(req.State)
	code := strings.TrimSpace(req.Code)
	if stateValue == "" || code == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth callback is invalid.", nil)
		return
	}
	state, ok := s.consumeCloudOAuthState(stateValue, time.Now().UTC())
	if !ok {
		s.writeAPIError(w, r, http.StatusUnauthorized, errorCodeUnauthorized, "OAuth state is invalid or expired.", nil)
		return
	}
	summary, err := s.completeCloudOAuthLogin(r.Context(), code)
	if err != nil {
		s.config.Logger.Warn("cloud oauth callback failed", "error", err)
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	if s.config.LocalDeviceStateDir != "" {
		if err := agentapp.SaveCloudBindingSummary(s.config.LocalDeviceStateDir, cloudBindingSummaryFromSession(summary), time.Now().UTC()); err != nil {
			s.config.Logger.Warn("persist local cloud session", "error", err)
			s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
			return
		}
	}
	s.setLocalCloudSession(summary)
	if s.config.OnLocalCloudSession != nil {
		s.config.OnLocalCloudSession(summary)
	}
	writeJSON(w, http.StatusOK, CloudOAuthCallbackResp{CloudSession: summary, Redirect: state.PostAuthRedirect})
}

func (s *Handler) handleLocalWorkspaces(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/agent-api/workspaces"), "/")
	parts := []string{"", "workspaces"}
	if path != "" {
		parts = append(parts, strings.Split(path, "/")...)
	}
	s.handleWorkspaceRoute(w, r, s.localRuntime, "", parts)
}

func (s *Handler) handleLocalSessions(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/agent-api/sessions" {
		s.writeNotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	var request terminalapp.CreateSessionReq
	if !s.decodeJSONRequest(w, r, &request) {
		return
	}
	s.handleJSONRuntimeWithStatus(w, r, s.localRuntime, "", "create_session", request, "", http.StatusCreated)
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
		var request terminalapp.UpdateWorkspaceOrderReq
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "workspace_order", request, "")
		return
	}
	if len(parts) == 3 && r.Method == http.MethodDelete {
		s.handleNoContentRuntime(w, r, endpoint, "delete_workspace", terminalapp.DeleteWorkspaceReq{WorkspaceId: parts[2]})
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
			s.handleJSONRuntime(w, r, endpoint, deviceId, "workspace_sessions", terminalapp.WorkspaceSessionsReq{WorkspaceId: workspaceId}, "")
		case http.MethodPost:
			var request terminalapp.CreateSessionReq
			if !s.decodeJSONRequest(w, r, &request) {
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
		var request terminalapp.UpdateSessionOrderReq
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "session_order", terminalapp.WorkspaceSessionOrderReq{WorkspaceId: workspaceId, SessionIds: request.SessionIds}, "")
		return
	}
	sessionId := parts[0]
	if sessionId == "" {
		s.writeNotFound(w, r)
		return
	}
	params := terminalapp.WorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleJSONRuntime(w, r, endpoint, deviceId, "get_session", params, "")
		case http.MethodPatch:
			var request terminalapp.UpdateSessionReq
			if !s.decodeJSONRequest(w, r, &request) {
				return
			}
			s.handleJSONRuntime(w, r, endpoint, deviceId, "update_session", terminalapp.UpdateWorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId, Request: request}, "")
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
		var request terminalapp.RerunSessionReq
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "rerun_session", terminalapp.RerunWorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId, Request: request}, "")
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
func writeTerminalControl(conn *websocket.Conn, message terminalproto.ServerMessage) error {
	data, err := terminalproto.EncodeServer(message)
	if err != nil {
		return err
	}
	return conn.Write(context.Background(), websocket.MessageText, data)
}

func (s *Handler) completeCloudOAuthLogin(ctx context.Context, code string) (CloudSessionSummary, error) {
	device := s.config.LocalDevice
	if device.Id == "" || device.Name == "" {
		return CloudSessionSummary{}, fmt.Errorf("local device identity is incomplete")
	}
	if strings.TrimSpace(device.PublicKey) == "" {
		return CloudSessionSummary{}, fmt.Errorf("local device public key is incomplete")
	}
	token, err := s.cloudOAuthConfig().Exchange(ctx, code)
	if err != nil {
		return CloudSessionSummary{}, err
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return CloudSessionSummary{}, fmt.Errorf("cloud OAuth token response missing access token")
	}
	reportBody, err := json.Marshal(CloudOAuthDeviceReportReq{Id: device.Id, Name: device.Name, PublicKey: device.PublicKey})
	if err != nil {
		return CloudSessionSummary{}, err
	}
	reportReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.CloudGateURL+"/cloud-api/devices/current", bytes.NewReader(reportBody))
	if err != nil {
		return CloudSessionSummary{}, err
	}
	reportReq.Header.Set("Content-Type", "application/json")
	reportReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	reportResp, err := http.DefaultClient.Do(reportReq)
	if err != nil {
		return CloudSessionSummary{}, err
	}
	defer func() { _ = reportResp.Body.Close() }()
	if reportResp.StatusCode < 200 || reportResp.StatusCode >= 300 {
		return CloudSessionSummary{}, fmt.Errorf("cloud device report status %d", reportResp.StatusCode)
	}
	return CloudSessionSummary{GateURL: s.config.CloudGateURL, DeviceId: device.Id, DeviceName: device.Name, ConnectedAt: time.Now().UTC()}, nil
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
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
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
	config.CloudGateURL = strings.TrimRight(strings.TrimSpace(config.CloudGateURL), "/")
	config.CORSAllowedOrigins = cleanOrigins(config.CORSAllowedOrigins)
	config.CloudOAuth.ClientID = strings.TrimSpace(config.CloudOAuth.ClientID)
	config.CloudOAuth.ClientSecret = strings.TrimSpace(config.CloudOAuth.ClientSecret)
	config.CloudOAuth.RedirectUrl = strings.TrimRight(strings.TrimSpace(config.CloudOAuth.RedirectUrl), "/")
	cleanScopes := make([]string, 0, len(config.CloudOAuth.Scopes))
	for _, scope := range config.CloudOAuth.Scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			cleanScopes = append(cleanScopes, scope)
		}
	}
	config.CloudOAuth.Scopes = cleanScopes
	return config
}
