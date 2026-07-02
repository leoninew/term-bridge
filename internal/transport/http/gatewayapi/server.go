package gatewayapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/oauth2"

	agentapp "termbridge-go/internal/application/agent"
	authapp "termbridge-go/internal/application/auth"
	terminalapp "termbridge-go/internal/application/terminal"
	devicerepo "termbridge-go/internal/infrastructure/repository/device"
	terminalproto "termbridge-go/internal/protocol/terminal"
	"termbridge-go/internal/protocol/tunnel"
	"termbridge-go/internal/transport/http/gatewayapi/auth"
)

type Config struct {
	Username               string
	Password               string
	JWTSecret              []byte
	DebugErrors            bool
	Logger                 *slog.Logger
	AuthService            *authapp.Service
	AgentTunnelAudience    string
	DevicePublicKeys       map[string]ed25519.PublicKey
	DeviceRepository       *devicerepo.Repository
	CloudGateURL           string
	CloudOAuth             CloudOAuthConfig
	CloudOAuthAttemptStore *authapp.CloudOAuthAttemptStore
	LocalDevice            agentapp.Device
	LocalRuntime           agentapp.RuntimeAccess
	ServerMode             string
	OnLocalCloudSession    func(CloudSessionSummary)
}

type CloudOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

type Handler struct {
	config             Config
	auth               *auth.Auther
	authService        *authapp.Service
	registry           *DeviceRegistry
	routes             map[string]*agentRoute
	routeMu            sync.Mutex
	writers            map[string]tunnel.StreamId
	writerMu           sync.Mutex
	cacheMu            sync.Mutex
	workspaceTreeCache map[string]json.RawMessage
	historyCache       map[string]map[string]string
	cloudSessionMu     sync.Mutex
	cloudSession       *CloudSessionSummary
	localRuntime       runtimeEndpoint
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
	config = normalizeConfig(config)
	handler := &Handler{config: config, auth: auth.NewAuther(auth.Credentials{Username: config.Username, Password: config.Password}, auth.NewTokenService(config.JWTSecret)), authService: config.AuthService, registry: NewDeviceRegistry(), routes: map[string]*agentRoute{}, writers: map[string]tunnel.StreamId{}, workspaceTreeCache: map[string]json.RawMessage{}, historyCache: map[string]map[string]string{}}
	if handler.isLocalServerMode() && config.LocalRuntime != nil {
		handler.localRuntime = localRuntimeEndpoint{runtime: config.LocalRuntime, handler: handler}
	}
	return handler
}

func (h *Handler) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", h.handleHealth)
	mux.HandleFunc("/api/auth/login", h.handleAuthLogin)
	mux.HandleFunc("/api/auth/logout", h.authMiddleware(http.HandlerFunc(h.handleLogout)).ServeHTTP)
	mux.HandleFunc("/api/auth/me", h.handleAuthMe)
	mux.HandleFunc("/api/auth/register", h.handleRegister)
	mux.HandleFunc("/api/auth/email/verify", h.handleVerifyEmail)
	mux.HandleFunc("/api/auth/email/verification/resend", h.handleResendVerification)
	mux.HandleFunc("/api/auth/password/change", h.authMiddleware(http.HandlerFunc(h.handleChangePassword)).ServeHTTP)
	mux.HandleFunc("/api/auth/password-reset/request", h.handlePasswordResetRequest)
	mux.HandleFunc("/api/auth/password-reset/confirm", h.handlePasswordResetConfirm)
	mux.HandleFunc("/api/auth/google", h.handleGoogleAuth)
	mux.HandleFunc("/api/auth/google/callback", h.handleGoogleCallback)
	mux.HandleFunc("/api/cloud-oauth/start", h.handleCloudOAuthStart)
	mux.HandleFunc("/api/cloud-oauth/callback", h.handleCloudOAuthCallback)
	mux.HandleFunc("/api/cloud-oauth/authorize", h.handleCloudOAuthAuthorize)
	mux.HandleFunc("/api/cloud-oauth/exchange", h.handleCloudOAuthExchange)
	mux.HandleFunc("/api/workspaces", h.authMiddleware(http.HandlerFunc(h.handleLocalWorkspaces)).ServeHTTP)
	mux.HandleFunc("/api/workspaces/", h.authMiddleware(http.HandlerFunc(h.handleLocalWorkspaces)).ServeHTTP)
	mux.HandleFunc("/api/sessions", h.authMiddleware(http.HandlerFunc(h.handleLocalSessions)).ServeHTTP)
	mux.HandleFunc("/api/devices", h.authMiddleware(http.HandlerFunc(h.handleDevices)).ServeHTTP)
	mux.HandleFunc("/api/devices/current", h.authMiddleware(http.HandlerFunc(h.handleCurrentDevice)).ServeHTTP)
	mux.HandleFunc("/api/devices/", h.authMiddleware(http.HandlerFunc(h.handleDevice)).ServeHTTP)
	mux.HandleFunc("/api/agent/tunnel", h.handleAgentTunnel)
	mux.HandleFunc("/api", h.writeNotFound)
	mux.HandleFunc("/api/", h.writeNotFound)
	return mux
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
	if !s.requireCloudServerMode(w, r) {
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
		capabilities := s.authService.Capabilities()
		capabilities.CloudOAuthEnabled = s.cloudOAuthEnabled()
		cloudSession := s.localCloudSessionSummary()
		claims, ok := s.claimsFromRequest(r)
		if !ok {
			writeJSON(w, http.StatusOK, AuthMeResp{Authenticated: false, Capabilities: &capabilities, CloudSession: cloudSession})
			return
		}
		user, err := s.authService.UserFromClaims(r.Context(), claims)
		if err != nil {
			s.writeUnauthorized(w, r)
			return
		}
		writeJSON(w, http.StatusOK, AuthMeResp{Authenticated: true, User: &user, Capabilities: &capabilities, CloudSession: cloudSession})
		return
	}
	writeJSON(w, http.StatusOK, AuthMeResp{Authenticated: true, Username: s.auth.UsernameFromRequest(r)})
}

func (s *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudServerMode(w, r) {
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
	if !s.requireCloudServerMode(w, r) {
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
	if !s.requireCloudServerMode(w, r) {
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
	if !s.requireCloudServerMode(w, r) {
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
	if !s.requireCloudServerMode(w, r) {
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
	if !s.requireCloudServerMode(w, r) {
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
	if !s.requireCloudServerMode(w, r) {
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
	if !s.requireCloudServerMode(w, r) {
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
		if s.isLocalServerMode() {
			next.ServeHTTP(w, r)
			return
		}
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
func (s *Handler) isLocalServerMode() bool {
	return s.config.ServerMode == "local"
}

func (s *Handler) cloudOAuthEnabled() bool {
	return s.isLocalServerMode() && s.config.CloudOAuthAttemptStore != nil && s.config.CloudGateURL != "" && s.config.CloudOAuth.ClientID != "" && s.config.CloudOAuth.RedirectURL != ""
}

func (s *Handler) localCloudSessionSummary() *CloudSessionSummary {
	if !s.isLocalServerMode() {
		return nil
	}
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

func (s *Handler) requireCloudServerMode(w http.ResponseWriter, r *http.Request) bool {
	if s.isLocalServerMode() {
		s.writeNotFound(w, r)
		return false
	}
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
	case errors.Is(err, authapp.ErrInvalidCredentials):
		s.writeUnauthorized(w, r)
	case errors.Is(err, authapp.ErrEmailNotVerified):
		s.writeAPIError(w, r, http.StatusForbidden, "email_not_verified", "Email is not verified.", nil)
	case errors.Is(err, authapp.ErrEmailAlreadyUsed):
		s.writeAPIError(w, r, http.StatusConflict, "email_already_used", "Email is already used.", nil)
	case errors.Is(err, authapp.ErrCodeInvalid):
		s.writeAPIError(w, r, http.StatusBadRequest, "code_invalid", "Code is invalid or expired.", nil)
	case errors.Is(err, authapp.ErrCodeCooldown):
		s.writeAPIError(w, r, http.StatusTooManyRequests, "code_cooldown", "Please wait before requesting another code.", nil)
	case errors.Is(err, authapp.ErrProviderUnsupported):
		s.writeAPIError(w, r, http.StatusBadRequest, "provider_unsupported", "Provider is unsupported for this operation.", nil)
	default:
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
	}
}

func (s *Handler) handleCloudOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	if !s.cloudOAuthEnabled() {
		s.writeNotFound(w, r)
		return
	}
	state, err := s.config.CloudOAuthAttemptStore.CreateWithOptions(authapp.CloudOAuthAttemptOptions{CallbackURL: s.config.CloudOAuth.RedirectURL, PostAuthRedirect: r.URL.Query().Get("redirect"), GateURL: s.config.CloudGateURL, DeviceId: s.config.LocalDevice.Id, DeviceName: s.config.LocalDevice.Name, TTL: authapp.CloudOAuthAttemptTTL})
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	oauthConfig := s.cloudOAuthConfig()
	writeJSON(w, http.StatusOK, CloudOAuthStartResp{AuthorizeURL: oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)})
}

func (s *Handler) cloudOAuthConfig() oauth2.Config {
	return oauth2.Config{
		ClientID:     s.config.CloudOAuth.ClientID,
		ClientSecret: s.config.CloudOAuth.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  s.config.CloudGateURL + "/oauth2/authorize",
			TokenURL: s.config.CloudGateURL + "/oauth2/token",
		},
		RedirectURL: s.config.CloudOAuth.RedirectURL,
		Scopes:      append([]string(nil), s.config.CloudOAuth.Scopes...),
	}
}

func (s *Handler) handleCloudOAuthExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if s.isLocalServerMode() {
		s.writeNotFound(w, r)
		return
	}
	if s.authService == nil || s.config.DeviceRepository == nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Cloud connection exchange is not configured.", nil)
		return
	}
	var req CloudOAuthExchangeReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	userId, ok, err := s.config.DeviceRepository.UseBindingCode(r.Context(), req.Code)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	if !ok {
		s.writeUnauthorized(w, r)
		return
	}
	result, err := s.authService.IssueUserToken(r.Context(), userId)
	if err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, TokenResp{AccessToken: result.Token, TokenType: "bearer"})
}

func (s *Handler) handleCloudOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	if !s.requireCloudServerMode(w, r) {
		return
	}
	claims, ok := s.claimsFromRequest(r)
	if !ok || claims.Provider == authapp.ProviderLocalAdmin {
		s.writeUnauthorized(w, r)
		return
	}
	if s.config.DeviceRepository == nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Device repository is not configured.", nil)
		return
	}
	clientId := strings.TrimSpace(r.URL.Query().Get("client_id"))
	redirectURL := strings.TrimRight(strings.TrimSpace(r.URL.Query().Get("redirect_uri")), "/")
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if clientId == "" || clientId != s.config.CloudOAuth.ClientID || redirectURL == "" || redirectURL != s.config.CloudOAuth.RedirectURL || state == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth redirect is invalid.", nil)
		return
	}
	code, err := s.config.DeviceRepository.CreateBindingCode(r.Context(), claims.Sub, time.Now().UTC().Add(devicerepo.BindingCodeTTL))
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	redirectURL, err = appendBindingCallback(redirectURL, code, state)
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth redirect is invalid.", err)
		return
	}
	writeJSON(w, http.StatusOK, CloudOAuthAuthorizeResp{RedirectURL: redirectURL})
}

func (s *Handler) handleCloudOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.cloudOAuthEnabled() {
		s.writeNotFound(w, r)
		return
	}
	var req CloudOAuthCallbackReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	state := strings.TrimSpace(req.State)
	code := strings.TrimSpace(req.Code)
	if state == "" || code == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth callback is invalid.", nil)
		return
	}
	attempt, ok, err := s.config.CloudOAuthAttemptStore.CompleteAttempt(state)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	if !ok {
		s.writeAPIError(w, r, http.StatusUnauthorized, errorCodeUnauthorized, "OAuth state is invalid or expired.", nil)
		return
	}
	summary, err := s.completeCloudOAuthLogin(r.Context(), code, attempt)
	if err != nil {
		s.config.Logger.Warn("cloud oauth callback failed", "error", err)
		s.writeAPIError(w, r, http.StatusBadGateway, errorCodeUpstream, errorMessageUpstream, err)
		return
	}
	s.setLocalCloudSession(summary)
	if s.config.OnLocalCloudSession != nil {
		s.config.OnLocalCloudSession(summary)
	}
	writeJSON(w, http.StatusOK, CloudOAuthCallbackResp{CloudSession: summary, Redirect: attempt.PostAuthRedirect})
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
	device := devicerepo.Device{ID: req.ID, Name: req.Name, PublicKey: req.PublicKey}
	if err := s.config.DeviceRepository.UpsertUserDevice(r.Context(), claims.Sub, device); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Device report is invalid.", err)
		return
	}
	writeJSON(w, http.StatusOK, CurrentDeviceResp{Accepted: true, Device: s.deviceSummary(device)})
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
	devices, err := s.devicesForRequest(r)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	writeJSON(w, http.StatusOK, ListDevicesResp{Items: devices})
}
func (s *Handler) handleLocalWorkspaces(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalServerMode() {
		s.writeNotFound(w, r)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/workspaces"), "/")
	parts := []string{"", "workspaces"}
	if path != "" {
		parts = append(parts, strings.Split(path, "/")...)
	}
	s.handleWorkspaceRoute(w, r, s.localRuntime, "", parts)
}

func (s *Handler) handleLocalSessions(w http.ResponseWriter, r *http.Request) {
	if !s.isLocalServerMode() {
		s.writeNotFound(w, r)
		return
	}
	if r.URL.Path != "/api/sessions" {
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

func (s *Handler) handleDevice(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/devices/"), "/")
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
			var request terminalapp.CreateSessionReq
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
func (s *Handler) handleDeleteDevice(w http.ResponseWriter, r *http.Request, deviceId string) {
	claims, ok := s.claimsFromRequest(r)
	if !ok || claims.Provider == authapp.ProviderLocalAdmin {
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
	cols, rows, hasAttachSize, sizeErr := terminalAttachSizeFromQuery(r)
	if sizeErr != nil {
		s.config.Logger.Warn("terminal attach size invalid", "workspace_id", workspaceId, "session_id", sessionId, "error", sizeErr)
		_ = writeTerminalControl(conn, terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: sizeErr.Error()})
		return
	}
	term := &terminalRelay{sessionId: sessionId, browser: conn, done: make(chan struct{}), logger: s.config.Logger}
	route.addTerminal(streamId, term)
	attachReq := tunnel.TerminalAttachReq{WorkspaceId: workspaceId, SessionId: sessionId, RequestId: s.requestIdFor(w, r)}
	if hasAttachSize {
		attachReq.Cols = cols
		attachReq.Rows = rows
	}
	attach, err := tunnel.NewFrame(streamId, tunnel.FrameTerminalAttach, attachReq)
	if err != nil {
		return
	}
	if err := route.writeFrame(r.Context(), attach); err != nil {
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
					resize, err := tunnel.NewFrame(streamId, tunnel.FrameTerminalResize, tunnel.TerminalResizePayload{Cols: message.Cols, Rows: message.Rows})
					if err == nil {
						_ = route.writeFrame(r.Context(), resize)
					}
				case terminalproto.TypeDetach:
					closeFrame, _ := tunnel.NewFrame(streamId, tunnel.FrameClose, nil)
					_ = route.writeFrame(context.Background(), closeFrame)
					closeOnce(term.done)
					return
				case terminalproto.TypePing:
					_ = writeTerminalControl(conn, terminalproto.ServerMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
				}
			case websocket.MessageBinary:
				input, err := tunnel.NewFrame(streamId, tunnel.FrameTerminalInput, tunnel.TerminalDataPayload{Data: data})
				if err == nil {
					_ = route.writeFrame(r.Context(), input)
				}
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
	if deviceId != "" && hello.DeviceId != deviceId {
		_ = conn.Close(websocket.StatusPolicyViolation, "device signature mismatch")
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
			if err == nil {
				_ = route.writeFrame(r.Context(), pong)
			}
		}
	}
}
func (s *Handler) verifyAgentTunnelRequest(r *http.Request) (string, bool) {
	if len(s.config.DevicePublicKeys) > 0 || s.config.DeviceRepository != nil {
		deviceId := strings.TrimSpace(r.Header.Get(agentapp.HeaderDeviceId))
		publicKey := s.config.DevicePublicKeys[deviceId]
		if len(publicKey) == 0 && s.config.DeviceRepository != nil {
			publicKeyText, err := s.config.DeviceRepository.PublicKey(r.Context(), deviceId)
			if err == nil {
				publicKey, _ = base64.StdEncoding.DecodeString(publicKeyText)
			}
		}
		if len(publicKey) == 0 {
			return "", false
		}
		verifiedDeviceId, err := agentapp.VerifySignedRequest(r, s.configAudience(), publicKey, time.Now())
		return verifiedDeviceId, err == nil
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
	return "termbridge-gateway"
}

func (s *Handler) devicesForRequest(r *http.Request) ([]DeviceSummary, error) {
	if s.authService == nil {
		return s.registry.List(), nil
	}
	if s.isLocalServerMode() {
		deviceId := strings.TrimSpace(s.config.LocalDevice.Id)
		if deviceId == "" {
			return s.registry.List(), nil
		}
		if runtimeDevice, ok := s.registry.Get(deviceId); ok {
			return []DeviceSummary{runtimeDevice}, nil
		}
		name := s.config.LocalDevice.Name
		if name == "" {
			name = deviceId
		}
		return []DeviceSummary{{Id: deviceId, Name: name, Online: false, Status: "offline"}}, nil
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

func (s *Handler) deviceSummary(device devicerepo.Device) DeviceSummary {
	if runtimeDevice, ok := s.registry.Get(device.ID); ok {
		if runtimeDevice.Name == "" {
			runtimeDevice.Name = device.Name
		}
		return normalizeDeviceStatus(runtimeDevice)
	}
	return DeviceSummary{Id: device.ID, Name: device.Name, Online: false, Status: "offline", LastSeen: device.UpdatedAt}
}

func (s *Handler) completeCloudOAuthLogin(ctx context.Context, code string, attempt authapp.CloudOAuthAttempt) (CloudSessionSummary, error) {
	device := s.config.LocalDevice
	if attempt.DeviceId != "" {
		device.Id = attempt.DeviceId
	}
	if attempt.DeviceName != "" {
		device.Name = attempt.DeviceName
	}
	if device.Id == "" || device.Name == "" {
		return CloudSessionSummary{}, fmt.Errorf("local device identity is incomplete")
	}
	if strings.TrimSpace(device.PublicKey) == "" {
		return CloudSessionSummary{}, fmt.Errorf("local device public key is incomplete")
	}
	gateURL := attempt.GateURL
	if gateURL == "" {
		gateURL = s.config.CloudGateURL
	}
	body, err := json.Marshal(CloudOAuthExchangeTokenReq{Code: code})
	if err != nil {
		return CloudSessionSummary{}, err
	}
	exchangeURL := gateURL + "/api/cloud-oauth/exchange"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, exchangeURL, bytes.NewReader(body))
	if err != nil {
		return CloudSessionSummary{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CloudSessionSummary{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CloudSessionSummary{}, fmt.Errorf("cloud exchange status %d", resp.StatusCode)
	}
	var tokenResp CloudOAuthExchangeTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return CloudSessionSummary{}, err
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return CloudSessionSummary{}, fmt.Errorf("cloud exchange response missing access token")
	}
	reportBody, err := json.Marshal(CloudOAuthDeviceReportReq{ID: device.Id, Name: device.Name, PublicKey: device.PublicKey})
	if err != nil {
		return CloudSessionSummary{}, err
	}
	reportURL := gateURL + "/api/devices/current"
	reportReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reportURL, bytes.NewReader(reportBody))
	if err != nil {
		return CloudSessionSummary{}, err
	}
	reportReq.Header.Set("Content-Type", "application/json")
	reportReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	reportResp, err := http.DefaultClient.Do(reportReq)
	if err != nil {
		return CloudSessionSummary{}, err
	}
	defer reportResp.Body.Close()
	if reportResp.StatusCode < 200 || reportResp.StatusCode >= 300 {
		return CloudSessionSummary{}, fmt.Errorf("cloud device report status %d", reportResp.StatusCode)
	}
	return CloudSessionSummary{GateURL: gateURL, DeviceId: device.Id, DeviceName: device.Name, ConnectedAt: time.Now().UTC()}, nil
}

func appendBindingCallback(callbackURL string, code string, state string) (string, error) {
	parsed, err := url.Parse(callbackURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid callback URL")
	}
	query := parsed.Query()
	query.Set("code", code)
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
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
	return []string{"http://" + r.Host, "https://" + r.Host}
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
	config.ServerMode = strings.ToLower(strings.TrimSpace(config.ServerMode))
	if config.ServerMode == "" {
		config.ServerMode = "local"
	}
	config.CloudGateURL = strings.TrimRight(strings.TrimSpace(config.CloudGateURL), "/")
	config.CloudOAuth.ClientID = strings.TrimSpace(config.CloudOAuth.ClientID)
	config.CloudOAuth.ClientSecret = strings.TrimSpace(config.CloudOAuth.ClientSecret)
	config.CloudOAuth.RedirectURL = strings.TrimRight(strings.TrimSpace(config.CloudOAuth.RedirectURL), "/")
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
