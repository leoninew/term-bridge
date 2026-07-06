package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	tunnelv1 "termbridge-go/internal/gen/proto/termbridge/tunnel/v1"
	"termbridge-go/internal/shared/common/auth"
	terminalproto "termbridge-go/internal/shared/dto/protocol/terminal"
	"termbridge-go/internal/shared/dto/protocol/tunnel"
)

type Config struct {
	Username            string
	Password            string
	JWTSecret           []byte
	DebugErrors         bool
	Logger              *slog.Logger
	AuthService         AuthService
	AgentTunnelAudience string
	DeviceRepository    DeviceRepository
	CloudGateURL        string
	CORSAllowedOrigins  []string
}

type AuthService interface {
	Login(ctx context.Context, email, password string) (AuthResult, error)
	UserFromClaims(ctx context.Context, claims auth.Claims) (UserView, error)
	Register(ctx context.Context, email, password string) error
	VerifyEmail(ctx context.Context, email, code string) error
	ResendVerification(ctx context.Context, email string) error
	ChangePassword(ctx context.Context, userId, currentPassword, newPassword string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, email, code, newPassword string) error
	GoogleAuthURL(ctx context.Context) (string, error)
	GoogleCallback(ctx context.Context, code, state string) (AuthResult, error)
	IssueUserToken(ctx context.Context, userId string) (AuthResult, error)
	VerifyToken(token string) (auth.Claims, error)
	VerifyBasic(ctx context.Context, username, password string) bool
}

type Handler struct {
	config             Config
	auth               *auth.Auther
	authService        AuthService
	registry           *DeviceRegistry
	routes             map[string]*agentRoute
	routeMu            sync.Mutex
	writers            map[string]string
	writerMu           sync.Mutex
	cacheMu            sync.Mutex
	workspaceTreeCache map[string]json.RawMessage
	historyCache       map[string]map[string]string
}

type DeviceRegistry struct {
	mu      sync.Mutex
	devices map[string]DeviceSummary
}

func NewDeviceRegistry() *DeviceRegistry { return &DeviceRegistry{devices: map[string]DeviceSummary{}} }
func (r *DeviceRegistry) Register(id string, name string, now time.Time) DeviceSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := r.devices[id]
	if d.ConnectedAt.IsZero() {
		d.ConnectedAt = now
	}
	d.Id = id
	d.Name = name
	d.Online = true
	d.Status = "online"
	d.LastSeen = now
	r.devices[id] = d
	return d
}
func (r *DeviceRegistry) MarkOffline(id string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.devices[id]
	if !ok {
		return
	}
	d.Online = false
	d.Status = "offline"
	d.LastSeen = now
	r.devices[id] = d
}
func (r *DeviceRegistry) List() []DeviceSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]DeviceSummary, 0, len(r.devices))
	for _, d := range r.devices {
		out = append(out, normalizeDeviceStatus(d))
	}
	return out
}

func (r *DeviceRegistry) Get(id string) (DeviceSummary, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.devices[id]
	return normalizeDeviceStatus(d), ok
}

func (r *DeviceRegistry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.devices, id)
}

func normalizeDeviceStatus(d DeviceSummary) DeviceSummary {
	if d.Status == "" {
		if d.Online {
			d.Status = "online"
		} else {
			d.Status = "offline"
		}
	}
	return d
}

func New(config Config) *Handler {
	return newHandler(config)
}

func newHandler(config Config) *Handler {
	config = normalizeConfig(config)
	handler := &Handler{config: config, auth: auth.NewAuther(auth.Credentials{Username: config.Username, Password: config.Password}, auth.NewTokenService(config.JWTSecret)), authService: config.AuthService, registry: NewDeviceRegistry(), routes: map[string]*agentRoute{}, writers: map[string]string{}, workspaceTreeCache: map[string]json.RawMessage{}, historyCache: map[string]map[string]string{}}
	return handler
}

func (h *Handler) Handler() http.Handler {
	mux := http.NewServeMux()
	h.registerCommonRoutes(mux)
	h.registerCloudRoutes(mux)
	mux.HandleFunc("/cloud-api", h.writeNotFound)
	mux.HandleFunc("/cloud-api/", h.writeNotFound)
	return mux
}

func (h *Handler) registerCommonRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/cloud-api/health", h.handleHealth)
	mux.HandleFunc("/cloud-api/auth/logout", h.authMiddleware(http.HandlerFunc(h.handleLogout)).ServeHTTP)
	mux.HandleFunc("/cloud-api/auth/me", h.handleAuthMe)
}

func (h *Handler) registerCloudRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/cloud-api/auth/login", h.handleAuthLogin)
	mux.HandleFunc("/cloud-api/auth/register", h.handleRegister)
	mux.HandleFunc("/cloud-api/auth/email/verify", h.handleVerifyEmail)
	mux.HandleFunc("/cloud-api/auth/email/verification/resend", h.handleResendVerification)
	mux.HandleFunc("/cloud-api/auth/password/change", h.authMiddleware(http.HandlerFunc(h.handleChangePassword)).ServeHTTP)
	mux.HandleFunc("/cloud-api/auth/password-reset/request", h.handlePasswordResetRequest)
	mux.HandleFunc("/cloud-api/auth/password-reset/confirm", h.handlePasswordResetConfirm)
	mux.HandleFunc("/cloud-api/auth/google", h.handleGoogleAuth)
	mux.HandleFunc("/cloud-api/auth/google/callback", h.handleGoogleCallback)
	mux.HandleFunc("/cloud-api/devices", h.authMiddleware(http.HandlerFunc(h.handleDevices)).ServeHTTP)
	mux.HandleFunc("/cloud-api/devices/current", h.authMiddleware(http.HandlerFunc(h.handleCurrentDevice)).ServeHTTP)
	mux.HandleFunc("/cloud-api/devices/", h.authMiddleware(http.HandlerFunc(h.handleDevice)).ServeHTTP)
	mux.HandleFunc("/cloud-api/agent/tunnel", h.handleAgentTunnel)
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
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req AuthLoginReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	login := req.Email
	if login == "" {
		login = req.Username
	}
	if s.authService != nil {
		result, err := s.authService.Login(r.Context(), login, req.Password)
		if err != nil {
			s.writeAuthError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, TokenResp{AccessToken: result.Token, TokenType: "bearer"})
		return
	}
	if !s.auth.ValidCredentials(login, req.Password) {
		s.writeUnauthorized(w, r)
		return
	}
	token, err := s.auth.SignToken(login)
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
		claims, ok := s.claimsFromRequest(r)
		if !ok {
			writeJSON(w, http.StatusOK, AuthMeResp{Authenticated: false})
			return
		}
		user, err := s.authService.UserFromClaims(r.Context(), claims)
		if err != nil {
			s.writeUnauthorized(w, r)
			return
		}
		writeJSON(w, http.StatusOK, AuthMeResp{Authenticated: true, User: &user})
		return
	}
	writeJSON(w, http.StatusOK, AuthMeResp{Authenticated: true, Username: s.auth.UsernameFromRequest(r)})
}

func (s *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req AuthRegisterReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	if s.authService == nil {
		s.writeNotFound(w, r)
		return
	}
	if err := s.authService.Register(r.Context(), req.Email, req.Password); err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Handler) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req AuthVerifyEmailReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	if s.authService == nil {
		s.writeNotFound(w, r)
		return
	}
	if err := s.authService.VerifyEmail(r.Context(), req.Email, req.Code); err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Handler) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req AuthResendVerificationReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	if s.authService == nil {
		s.writeNotFound(w, r)
		return
	}
	if err := s.authService.ResendVerification(r.Context(), req.Email); err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Handler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req AuthChangePasswordReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	claims, _ := s.claimsFromRequest(r)
	if err := s.authService.ChangePassword(r.Context(), claims.Sub, req.CurrentPassword, req.NewPassword); err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Handler) handlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req AuthPasswordResetRequestReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	if s.authService != nil {
		_ = s.authService.RequestPasswordReset(r.Context(), req.Email)
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Handler) handlePasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req AuthPasswordResetConfirmReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	if s.authService == nil {
		s.writeNotFound(w, r)
		return
	}
	if err := s.authService.ConfirmPasswordReset(r.Context(), req.Email, req.Code, req.NewPassword); err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Handler) handleGoogleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	if s.authService == nil {
		s.writeNotFound(w, r)
		return
	}
	url, err := s.authService.GoogleAuthURL(r.Context())
	if err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, GoogleAuthURLResp{AuthURL: url})
}
func (s *Handler) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req AuthGoogleCallbackReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	if s.authService == nil {
		s.writeNotFound(w, r)
		return
	}
	result, err := s.authService.GoogleCallback(r.Context(), req.Code, req.State)
	if err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, TokenResp{AccessToken: result.Token, TokenType: "bearer"})
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
func (s *Handler) requireCloudEndpoint(w http.ResponseWriter, r *http.Request) bool {
	_, _ = w, r
	return true
}

func (s *Handler) claimsFromRequest(r *http.Request) (auth.Claims, bool) {
	token := auth.ExtractBearerToken(r)
	if token == "" || s.authService == nil {
		return auth.Claims{}, false
	}
	claims, err := s.authService.VerifyToken(token)
	return claims, err == nil
}
func (s *Handler) writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		s.writeUnauthorized(w, r)
	case errors.Is(err, ErrEmailNotVerified):
		s.writeAPIError(w, r, http.StatusForbidden, "email_not_verified", "Email is not verified.", nil)
	case errors.Is(err, ErrEmailAlreadyUsed):
		s.writeAPIError(w, r, http.StatusConflict, "email_already_used", "Email is already used.", nil)
	case errors.Is(err, ErrCodeInvalid):
		s.writeAPIError(w, r, http.StatusBadRequest, "code_invalid", "Code is invalid or expired.", nil)
	case errors.Is(err, ErrCodeCooldown):
		s.writeAPIError(w, r, http.StatusTooManyRequests, "code_cooldown", "Please wait before requesting another code.", nil)
	case errors.Is(err, ErrProviderUnsupported):
		s.writeAPIError(w, r, http.StatusBadRequest, "provider_unsupported", "Provider is unsupported for this operation.", nil)
	default:
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
	}
}

func (s *Handler) handleCurrentDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if s.config.DeviceRepository == nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Device reporting is not configured.", nil)
		return
	}
	claims, ok := s.claimsFromRequest(r)
	if !ok {
		s.writeUnauthorized(w, r)
		return
	}
	var req CurrentDeviceReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	device := Device{Id: req.Id, Name: req.Name, PublicKey: req.PublicKey}
	if err := s.config.DeviceRepository.UpsertUserDevice(r.Context(), claims.Sub, device); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Device report is invalid.", err)
		return
	}
	writeJSON(w, http.StatusOK, CurrentDeviceResp{Accepted: true, Device: s.deviceSummary(device)})
}

func (s *Handler) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/cloud-api/devices" {
		s.writeNotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	devices, err := s.devicesForRequest(r)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	writeJSON(w, http.StatusOK, ListDevicesResp{Items: devices})
}
func (s *Handler) handleDevice(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/cloud-api/devices/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodDelete {
		s.handleDeleteDevice(w, r, parts[0])
		return
	}
	if len(parts) < 2 || parts[0] == "" {
		s.writeNotFound(w, r)
		return
	}
	deviceId := parts[0]
	route := s.routeFor(deviceId)
	endpoint := tunnelRuntimeEndpoint{route: route, deviceId: deviceId, handler: s}
	switch parts[1] {
	case "workspaces":
		s.handleWorkspaceRoute(w, r, endpoint, deviceId, parts)
	case "sessions":
		if len(parts) == 2 && r.Method == http.MethodPost {
			var request CreateSessionReq
			if !s.decodeJSONRequest(w, r, &request) {
				return
			}
			s.handleJSONRuntimeWithStatus(w, r, endpoint, deviceId, "create_session", request, "", http.StatusCreated)
			return
		}
		s.writeNotFound(w, r)
	default:
		s.writeNotFound(w, r)
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
		var request UpdateWorkspaceOrderReq
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "workspace_order", request, "")
		return
	}
	if len(parts) == 3 && r.Method == http.MethodDelete {
		s.handleNoContentRuntime(w, r, endpoint, "delete_workspace", DeleteWorkspaceReq{WorkspaceId: parts[2]})
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
			s.handleJSONRuntime(w, r, endpoint, deviceId, "workspace_sessions", WorkspaceSessionsReq{WorkspaceId: workspaceId}, "")
		case http.MethodPost:
			var request CreateSessionReq
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
		var request UpdateSessionOrderReq
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "session_order", WorkspaceSessionOrderReq{WorkspaceId: workspaceId, SessionIds: request.SessionIds}, "")
		return
	}
	sessionId := parts[0]
	if sessionId == "" {
		s.writeNotFound(w, r)
		return
	}
	params := WorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleJSONRuntime(w, r, endpoint, deviceId, "get_session", params, "")
		case http.MethodPatch:
			var request UpdateSessionReq
			if !s.decodeJSONRequest(w, r, &request) {
				return
			}
			s.handleJSONRuntime(w, r, endpoint, deviceId, "update_session", UpdateWorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId, Request: request}, "")
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
		var request RerunSessionReq
		if !s.decodeJSONRequest(w, r, &request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "rerun_session", RerunWorkspaceSessionReq{WorkspaceId: workspaceId, SessionId: sessionId, Request: request}, "")
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
func (s *Handler) handleDeleteDevice(w http.ResponseWriter, r *http.Request, deviceId string) {
	claims, ok := s.claimsFromRequest(r)
	if !ok || claims.Provider == ProviderLocalAdmin {
		s.writeUnauthorized(w, r)
		return
	}
	if s.config.DeviceRepository == nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Device repository is not configured.", nil)
		return
	}
	owns, err := s.config.DeviceRepository.UserOwnsDevice(r.Context(), claims.Sub, deviceId)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	if !owns {
		s.writeNotFound(w, r)
		return
	}
	if err := s.config.DeviceRepository.DeleteUserDevice(r.Context(), claims.Sub, deviceId); err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	s.disconnectDevice(deviceId, "device deleted")
	w.WriteHeader(http.StatusNoContent)
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

func (s *Handler) handleTerminalWS(w http.ResponseWriter, r *http.Request, route *agentRoute, workspaceId string, sessionId string) {
	writerKey := sessionScopeKey(workspaceId, sessionId)
	s.writerMu.Lock()
	if _, exists := s.writers[writerKey]; exists {
		s.writerMu.Unlock()
		s.writeAPIError(w, r, http.StatusConflict, errorCodeConflict, errorMessageConflict, nil)
		return
	}
	streamId := "term-" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	cols, rows, hasAttachSize, sizeErr := terminalAttachSizeFromQuery(r)
	if sizeErr != nil {
		s.config.Logger.Warn("terminal attach size invalid", "workspace_id", workspaceId, "session_id", sessionId, "error", sizeErr)
		_ = writeTerminalControl(conn, &terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: sizeErr.Error()})
		return
	}
	term := &terminalRelay{sessionId: sessionId, browser: conn, done: make(chan struct{}), logger: s.config.Logger}
	route.addTerminal(streamId, term)
	attachReq := &tunnelv1.TerminalAttachReq{WorkspaceId: workspaceId, SessionId: sessionId}
	if hasAttachSize {
		attachReq.Cols = int32(cols)
		attachReq.Rows = int32(rows)
	}
	attach := &tunnelv1.TunnelFrame{StreamId: streamId, RequestId: s.requestIdFor(w, r), Payload: &tunnelv1.TunnelFrame_TerminalAttach{TerminalAttach: attachReq}}
	if err := route.writeFrame(r.Context(), attach); err != nil {
		return
	}
	go func() {
		for {
			messageType, data, err := conn.Read(r.Context())
			if err != nil {
				closeFrame := &tunnelv1.TunnelFrame{StreamId: streamId, Payload: &tunnelv1.TunnelFrame_Close{Close: &tunnelv1.Close{Reason: "browser_disconnected"}}}
				_ = route.writeFrame(context.Background(), closeFrame)
				closeOnce(term.done)
				return
			}
			switch messageType {
			case websocket.MessageText:
				message, err := terminalproto.DecodeClient(data)
				if err != nil {
					_ = writeTerminalControl(conn, &terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: err.Error()})
					continue
				}
				switch message.Type {
				case terminalproto.TypeHello:
					continue
				case terminalproto.TypeResize:
					resize := &tunnelv1.TunnelFrame{StreamId: streamId, Payload: &tunnelv1.TunnelFrame_TerminalResize{TerminalResize: &tunnelv1.TerminalResize{Cols: message.Cols, Rows: message.Rows}}}
					_ = route.writeFrame(r.Context(), resize)
				case terminalproto.TypeDetach:
					closeFrame := &tunnelv1.TunnelFrame{StreamId: streamId, Payload: &tunnelv1.TunnelFrame_Close{Close: &tunnelv1.Close{Reason: "browser_detached"}}}
					_ = route.writeFrame(context.Background(), closeFrame)
					closeOnce(term.done)
					return
				case terminalproto.TypePing:
					_ = writeTerminalControl(conn, &terminalproto.ServerMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
				}
			case websocket.MessageBinary:
				input := &tunnelv1.TunnelFrame{StreamId: streamId, Payload: &tunnelv1.TunnelFrame_TerminalInput{TerminalInput: &tunnelv1.TerminalInput{Data: data}}}
				_ = route.writeFrame(r.Context(), input)
			}
		}
	}()
	<-term.done
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
func writeTerminalControl(conn *websocket.Conn, message *terminalproto.ServerMessage) error {
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
	deviceId, valid := s.verifyAgentTunnelRequest(r)
	if !valid {
		s.writeUnauthorized(w, r)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(tunnel.MaxFrameBytes)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	_, data, err := conn.Read(r.Context())
	if err != nil {
		return
	}
	frame, err := tunnel.UnmarshalFrame(data)
	if err != nil || frame.GetHello() == nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "expected hello")
		return
	}
	hello := frame.GetHello()
	if hello.GetDeviceId() == "" || hello.GetDeviceName() == "" {
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid hello")
		return
	}
	if deviceId != "" && hello.GetDeviceId() != deviceId {
		_ = conn.Close(websocket.StatusPolicyViolation, "device signature mismatch")
		return
	}
	s.registry.Register(hello.GetDeviceId(), hello.GetDeviceName(), time.Now().UTC())
	route := newAgentRoute(hello.GetDeviceId(), conn)
	s.setRoute(hello.GetDeviceId(), route)
	defer func() {
		s.clearRoute(hello.GetDeviceId(), route)
		route.closeTerminals("device disconnected")
		s.registry.MarkOffline(hello.GetDeviceId(), time.Now().UTC())
	}()
	ack := &tunnelv1.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &tunnelv1.TunnelFrame_HelloAck{HelloAck: &tunnelv1.HelloAck{ProtocolVersion: tunnel.ProtocolVersion}}}
	if err := route.writeFrame(r.Context(), ack); err != nil {
		return
	}
	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		frame, err := tunnel.UnmarshalFrame(data)
		if err != nil {
			continue
		}
		if route.dispatch(frame) {
			continue
		}
		if ping := frame.GetPing(); ping != nil {
			pong := &tunnelv1.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &tunnelv1.TunnelFrame_Pong{Pong: &tunnelv1.Pong{Nonce: ping.GetNonce()}}}
			_ = route.writeFrame(r.Context(), pong)
		}
	}
}
func (s *Handler) verifyAgentTunnelRequest(r *http.Request) (string, bool) {
	if s.config.DeviceRepository != nil {
		return verifyRepositorySignedTunnelRequest(r.Context(), r, s.config.DeviceRepository, s.configAudience())
	}
	username, password, ok := r.BasicAuth()
	if !ok {
		return "", false
	}
	if s.authService != nil {
		return "", s.authService.VerifyBasic(r.Context(), username, password)
	}
	return "", s.auth.ValidCredentials(username, password)
}

func (s *Handler) configAudience() string {
	if strings.TrimSpace(s.config.AgentTunnelAudience) != "" {
		return s.config.AgentTunnelAudience
	}
	return "termbridge-cloud"
}

func (s *Handler) devicesForRequest(r *http.Request) ([]DeviceSummary, error) {
	if s.authService == nil {
		return s.registry.List(), nil
	}
	claims, ok := s.claimsFromRequest(r)
	if !ok {
		return nil, nil
	}
	if s.config.DeviceRepository == nil {
		return s.registry.List(), nil
	}
	devices, err := s.config.DeviceRepository.ListDevicesForUser(r.Context(), claims.Sub)
	if err != nil {
		return nil, err
	}
	summaries := make([]DeviceSummary, 0, len(devices))
	for _, device := range devices {
		summaries = append(summaries, s.deviceSummary(device))
	}
	return summaries, nil
}

func (s *Handler) deviceSummary(device Device) DeviceSummary {
	if runtimeDevice, ok := s.registry.Get(device.Id); ok {
		if runtimeDevice.Name == "" {
			runtimeDevice.Name = device.Name
		}
		return normalizeDeviceStatus(runtimeDevice)
	}
	return DeviceSummary{Id: device.Id, Name: device.Name, Online: false, Status: "offline", LastSeen: device.UpdatedAt}
}

func (s *Handler) disconnectDevice(deviceId string, reason string) {
	s.routeMu.Lock()
	route := s.routes[deviceId]
	delete(s.routes, deviceId)
	s.routeMu.Unlock()
	if route != nil {
		route.closeTerminals(reason)
		if route.conn != nil {
			_ = route.conn.Close(websocket.StatusGoingAway, reason)
		}
	}
	s.registry.MarkOffline(deviceId, time.Now().UTC())
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
		panic("cloud api logger is required")
	}
	config.CloudGateURL = strings.TrimRight(strings.TrimSpace(config.CloudGateURL), "/")
	config.CORSAllowedOrigins = cleanOrigins(config.CORSAllowedOrigins)
	return config
}
