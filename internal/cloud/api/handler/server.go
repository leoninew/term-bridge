package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
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

	authmodel "gitee.com/leoninew/TermBridge-go/internal/cloud/model/user/auth"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	cloudproto "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	quotapkg "gitee.com/leoninew/TermBridge-go/internal/shared/application/quota"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/prototime"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
	workspacefs "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/workspacefs"

	"github.com/coder/websocket"
)

type Config struct {
	DebugErrors         bool
	Logger              *slog.Logger
	AuthService         AuthService
	AgentTunnelAudience string
	DeviceRepository    DeviceRepository
	QuotaRepository     QuotaRepository
	AttachQuota         *quotapkg.AttachCounter
	ConcurrentAttaches  int
	AdminUserIds        []string
	AdminEmails         []string
	CloudPublicURL      string
	CloudOAuth          CloudOAuthConfig
	CORSAllowedOrigins  []string
	Turnstile           TurnstileConfig
	CSRF                CSRFConfig
}

type QuotaRepository interface {
	GetLimit(ctx context.Context, userID, key string) (int, bool, error)
	UpsertLimit(ctx context.Context, userID, key string, value int, updatedBy string) error
	DeleteLimit(ctx context.Context, userID, key string) error
}

type TurnstileConfig struct {
	SiteKey          string
	SecretKey        string
	ExpectedHostname string
	Verify           TurnstileVerifier
}

type CSRFConfig struct {
	Tokens CSRFTokenService
}

type TurnstileVerifier interface {
	Verify(ctx context.Context, token string, remoteIP string) error
}

type CSRFTokenService interface {
	Issue() (string, error)
	Consume(token string) bool
}

type CloudOAuthConfig struct {
	Clients []CloudOAuthClientConfig
}

type CloudOAuthClientConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
	Scopes       []string
}

type AuthService interface {
	Login(ctx context.Context, email, password string) (*cloudproto.AuthLoginResp, error)
	UserFromClaims(ctx context.Context, claims auth.Claims) (*cloudproto.User, error)
	Register(ctx context.Context, email, password string) error
	VerifyEmail(ctx context.Context, email, code string) error
	ResendVerification(ctx context.Context, email string) error
	ChangePassword(ctx context.Context, userId, currentPassword, newPassword string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, email, code, newPassword string) error
	ExternalAuthURL(ctx context.Context, providerId string) (string, error)
	ExternalCallback(ctx context.Context, providerId, code, state string) (*cloudproto.AuthLoginResp, error)
	IssueUserToken(ctx context.Context, userId string) (*cloudproto.CloudOAuthTokenResp, error)
	VerifyToken(token string) (auth.Claims, error)
}

type Handler struct {
	config             Config
	authService        AuthService
	registry           *DeviceRegistry
	routes             map[string]*agentRoute
	routeMu            sync.Mutex
	writers            map[string]string
	writerMu           sync.Mutex
	cacheMu            sync.Mutex
	workspaceTreeCache map[string]json.RawMessage
	historyCache       map[string]map[string]string
	oauthCodeMu        sync.Mutex
	oauthCodes         map[string]cloudOAuthCode
	attachQuota        *quotapkg.AttachCounter
}

type cloudOAuthCode struct {
	ClientId    string
	RedirectUrl string
	UserId      string
	ExpiresAt   time.Time
}

const cloudOAuthCodeTtl = 10 * time.Minute

type DeviceRegistry struct {
	mu      sync.Mutex
	devices map[string]*cloudproto.DeviceSummary
}

func NewDeviceRegistry() *DeviceRegistry {
	return &DeviceRegistry{devices: map[string]*cloudproto.DeviceSummary{}}
}
func (r *DeviceRegistry) Register(id string, name string, now time.Time) *cloudproto.DeviceSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := cloneDeviceSummary(r.devices[id])
	if d.GetConnectedAt() == nil {
		d.ConnectedAt = prototime.FromTime(now)
	}
	d.Id = id
	d.Name = name
	d.Online = true
	d.Status = "online"
	d.LastSeen = prototime.FromTime(now)
	r.devices[id] = d
	return cloneDeviceSummary(d)
}
func (r *DeviceRegistry) MarkOffline(id string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := r.devices[id]
	if d == nil {
		return
	}
	d.Online = false
	d.Status = "offline"
	d.LastSeen = prototime.FromTime(now)
}
func (r *DeviceRegistry) List() []*cloudproto.DeviceSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*cloudproto.DeviceSummary, 0, len(r.devices))
	for _, d := range r.devices {
		out = append(out, normalizeDeviceStatus(cloneDeviceSummary(d)))
	}
	return out
}

func (r *DeviceRegistry) Get(id string) (*cloudproto.DeviceSummary, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := r.devices[id]
	return normalizeDeviceStatus(cloneDeviceSummary(d)), d != nil
}

func (r *DeviceRegistry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.devices, id)
}

func normalizeDeviceStatus(d *cloudproto.DeviceSummary) *cloudproto.DeviceSummary {
	if d == nil {
		return nil
	}
	if d.Status == "" {
		if d.Online {
			d.Status = "online"
		} else {
			d.Status = "offline"
		}
	}
	return d
}

func cloneDeviceSummary(d *cloudproto.DeviceSummary) *cloudproto.DeviceSummary {
	if d == nil {
		return &cloudproto.DeviceSummary{}
	}
	return &cloudproto.DeviceSummary{Id: d.GetId(), Name: d.GetName(), Online: d.GetOnline(), Status: d.GetStatus(), ConnectedAt: d.GetConnectedAt(), LastSeen: d.GetLastSeen()}
}

func New(config Config) *Handler {
	return newHandler(config)
}

func newHandler(config Config) *Handler {
	config = normalizeConfig(config)
	if config.AttachQuota == nil {
		config.AttachQuota = quotapkg.NewAttachCounter()
	}
	if config.ConcurrentAttaches <= 0 {
		config.ConcurrentAttaches = quotapkg.DefaultConcurrentAttaches
	}
	handler := &Handler{config: config, authService: config.AuthService, registry: NewDeviceRegistry(), routes: map[string]*agentRoute{}, writers: map[string]string{}, workspaceTreeCache: map[string]json.RawMessage{}, historyCache: map[string]map[string]string{}, oauthCodes: map[string]cloudOAuthCode{}, attachQuota: config.AttachQuota}
	return handler
}

func (h *Handler) Handler() http.Handler {
	mux := http.NewServeMux()
	h.registerCommonRoutes(mux)
	h.registerCloudRoutes(mux)
	mux.HandleFunc("/api", h.writeNotFound)
	mux.HandleFunc("/api/", h.writeNotFound)
	return mux
}

func (h *Handler) registerCommonRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/health", h.handleHealth)
	mux.HandleFunc("/api/auth/logout", h.authMiddleware(http.HandlerFunc(h.handleLogout)).ServeHTTP)
	mux.HandleFunc("/api/auth/me", h.handleAuthMe)
	mux.HandleFunc("/api/me/quota", h.authMiddleware(http.HandlerFunc(h.handleMyQuota)).ServeHTTP)
	mux.HandleFunc("/api/admin/users/", h.authMiddleware(http.HandlerFunc(h.handleAdminUserQuota)).ServeHTTP)
	mux.HandleFunc("/api/auth/turnstile/config", h.handleTurnstileConfig)
	mux.HandleFunc("/api/auth/login/csrf", h.handleLoginCSRF)
}

func (h *Handler) registerCloudRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/login", h.handleAuthLogin)
	mux.HandleFunc("/api/auth/register", h.handleRegister)
	mux.HandleFunc("/api/auth/email/verify", h.handleVerifyEmail)
	mux.HandleFunc("/api/auth/email/verification/resend", h.handleResendVerification)
	mux.HandleFunc("/api/auth/password/change", h.authMiddleware(http.HandlerFunc(h.handleChangePassword)).ServeHTTP)
	mux.HandleFunc("/api/auth/password-reset/request", h.handlePasswordResetRequest)
	mux.HandleFunc("/api/auth/password-reset/confirm", h.handlePasswordResetConfirm)
	mux.HandleFunc("/api/oauth2/", h.handleExternalOAuthCallback)
	mux.HandleFunc("/api/oauth2/google", h.handleGoogleExternalOAuth)
	mux.HandleFunc("/api/oauth2/github", h.handleGitHubExternalOAuth)
	mux.HandleFunc("/api/oauth2/authorize", h.authMiddleware(http.HandlerFunc(h.handleOAuthAuthorize)).ServeHTTP)
	mux.HandleFunc("/api/oauth2/token", h.handleOAuthToken)
	mux.HandleFunc("/api/devices", h.authMiddleware(http.HandlerFunc(h.handleDevices)).ServeHTTP)
	mux.HandleFunc("/api/devices/current", h.authMiddleware(http.HandlerFunc(h.handleCurrentDevice)).ServeHTTP)
	mux.HandleFunc("/api/devices/", h.authMiddleware(http.HandlerFunc(h.handleDevice)).ServeHTTP)
	mux.HandleFunc("/api/agent/tunnel", h.handleAgentTunnel)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.Handler().ServeHTTP(w, r) }

func (s *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, &shared.HealthResp{Status: "ok"})
}

func (s *Handler) handleTurnstileConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, &cloudproto.AuthSecurityConfigResp{TurnstileSiteKey: s.config.Turnstile.SiteKey})
}

func (s *Handler) handleLoginCSRF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	if s.config.CSRF.Tokens == nil {
		s.writeAuthSecurityError(w, r)
		return
	}
	token, err := s.config.CSRF.Tokens.Issue()
	if err != nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeInternal, "Authentication security is temporarily unavailable.", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, &cloudproto.AuthCsrfTokenResp{Token: token})
}

func (s *Handler) consumeCSRFToken(token string) bool {
	return s.config.CSRF.Tokens != nil && s.config.CSRF.Tokens.Consume(token)
}

func (s *Handler) verifyTurnstile(ctx context.Context, token string) bool {
	return s.config.Turnstile.Verify != nil && s.config.Turnstile.Verify.Verify(ctx, token, "") == nil
}

func (s *Handler) writeAuthSecurityError(w http.ResponseWriter, r *http.Request) {
	s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Authentication security verification failed.", errAuthSecurityValidation)
}

func (s *Handler) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req cloudproto.AuthLoginReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	if !s.consumeCSRFToken(req.CsrfToken) || !s.verifyTurnstile(r.Context(), req.TurnstileToken) {
		s.writeAuthSecurityError(w, r)
		return
	}
	login := req.Email
	if login == "" {
		login = req.Username
	}
	result, err := s.authService.Login(r.Context(), login, req.Password)
	if err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
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
	claims, ok := s.claimsFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusOK, &cloudproto.AuthMeResp{Authenticated: false})
		return
	}
	user, err := s.authService.UserFromClaims(r.Context(), claims)
	if err != nil {
		s.writeUnauthorized(w, r)
		return
	}
	writeJSON(w, http.StatusOK, &cloudproto.AuthMeResp{Authenticated: true, User: user})
}

func (s *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req cloudproto.AuthRegisterReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	if !s.verifyTurnstile(r.Context(), req.TurnstileToken) {
		s.writeAuthSecurityError(w, r)
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
	var req cloudproto.AuthVerifyEmailReq
	if !s.decodeJSONRequest(w, r, &req) {
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
	var req cloudproto.AuthResendVerificationReq
	if !s.decodeJSONRequest(w, r, &req) {
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
	var req cloudproto.AuthChangePasswordReq
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
	var req cloudproto.AuthPasswordResetRequestReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	_ = s.authService.RequestPasswordReset(r.Context(), req.Email)
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
	var req cloudproto.AuthPasswordResetConfirmReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	if err := s.authService.ConfirmPasswordReset(r.Context(), req.Email, req.Code, req.NewPassword); err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Handler) handleGoogleExternalOAuth(w http.ResponseWriter, r *http.Request) {
	s.handleExternalAuthURL(w, r, "google")
}

func (s *Handler) handleGitHubExternalOAuth(w http.ResponseWriter, r *http.Request) {
	s.handleExternalAuthURL(w, r, "github")
}

func (s *Handler) handleExternalAuthURL(w http.ResponseWriter, r *http.Request, providerId string) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	authUrl, err := s.authService.ExternalAuthURL(r.Context(), providerId)
	if err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	writeJSONObject(w, http.StatusOK, map[string]string{"auth_url": authUrl})
}

func (s *Handler) handleExternalOAuthCallback(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/oauth2/")
	parts := strings.Split(path, "/")
	providerId := strings.TrimSpace(parts[0])
	if providerId == "" || len(parts) != 2 || parts[1] != "callback" {
		s.writeNotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudEndpoint(w, r) {
		return
	}
	var req cloudproto.AuthExternalCallbackReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	result, err := s.authService.ExternalCallback(r.Context(), providerId, req.Code, req.State)
	if err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Handler) handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodGet, http.MethodPost)
		return
	}
	if !s.requireCloudOAuth(w, r) {
		return
	}
	if r.URL.Query().Get("response_type") != "code" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth response_type must be code.", nil)
		return
	}
	clientId := strings.TrimSpace(r.URL.Query().Get("client_id"))
	redirectUrl := strings.TrimSpace(r.URL.Query().Get("redirect_uri"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	client, ok := s.cloudOAuthClient(clientId, redirectUrl)
	if !ok || state == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth authorization request is invalid.", nil)
		return
	}
	claims, ok := s.claimsFromRequest(r)
	if !ok {
		s.writeUnauthorized(w, r)
		return
	}
	code, err := randomCloudOAuthCode()
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	s.storeCloudOAuthCode(code, cloudOAuthCode{ClientId: clientId, RedirectUrl: client.RedirectUrl, UserId: claims.Sub, ExpiresAt: time.Now().UTC().Add(cloudOAuthCodeTtl)})
	callback, err := url.Parse(client.RedirectUrl)
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth redirect_uri is invalid.", err)
		return
	}
	query := callback.Query()
	query.Set("code", code)
	query.Set("state", state)
	callback.RawQuery = query.Encode()
	writeJSONObject(w, http.StatusOK, map[string]string{"redirect_url": callback.String()})
}

func (s *Handler) handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !s.requireCloudOAuth(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, errorMessageBadRequest, err)
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth grant_type must be authorization_code.", nil)
		return
	}
	clientId := strings.TrimSpace(r.PostForm.Get("client_id"))
	clientSecret := strings.TrimSpace(r.PostForm.Get("client_secret"))
	redirectUrl := strings.TrimSpace(r.PostForm.Get("redirect_uri"))
	code := strings.TrimSpace(r.PostForm.Get("code"))
	if !s.validCloudOAuthClientSecret(clientId, redirectUrl, clientSecret) {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth token request is invalid.", nil)
		return
	}
	storedCode, ok := s.consumeCloudOAuthCode(code, time.Now().UTC())
	if !ok || storedCode.ClientId != clientId || storedCode.RedirectUrl != redirectUrl {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "OAuth authorization code is invalid or expired.", nil)
		return
	}
	result, err := s.authService.IssueUserToken(r.Context(), storedCode.UserId)
	if err != nil {
		s.writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Handler) requireCloudOAuth(w http.ResponseWriter, r *http.Request) bool {
	if len(s.config.CloudOAuth.Clients) == 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Cloud OAuth is not configured.", nil)
		return false
	}
	return true
}

func (s *Handler) cloudOAuthClient(clientId string, redirectUrl string) (CloudOAuthClientConfig, bool) {
	for _, client := range s.config.CloudOAuth.Clients {
		if client.ClientId == clientId && client.RedirectUrl == redirectUrl {
			return client, true
		}
	}
	return CloudOAuthClientConfig{}, false
}

func (s *Handler) validCloudOAuthClientSecret(clientId string, redirectUrl string, clientSecret string) bool {
	client, ok := s.cloudOAuthClient(clientId, redirectUrl)
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(client.ClientSecret), []byte(clientSecret)) == 1
}

func (s *Handler) storeCloudOAuthCode(code string, value cloudOAuthCode) {
	s.oauthCodeMu.Lock()
	defer s.oauthCodeMu.Unlock()
	s.oauthCodes[code] = value
}

func (s *Handler) consumeCloudOAuthCode(code string, now time.Time) (cloudOAuthCode, bool) {
	s.oauthCodeMu.Lock()
	defer s.oauthCodeMu.Unlock()
	value, ok := s.oauthCodes[code]
	delete(s.oauthCodes, code)
	if !ok || !value.ExpiresAt.After(now) {
		return cloudOAuthCode{}, false
	}
	return value, true
}

func randomCloudOAuthCode() (string, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data[:]), nil
}

func (s *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.claimsFromRequest(r); !ok {
			s.writeUnauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r)
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
	case errors.Is(err, authmodel.ErrInvalidCredentials):
		s.writeUnauthorized(w, r)
	case errors.Is(err, authmodel.ErrEmailNotVerified):
		s.writeAPIError(w, r, http.StatusForbidden, "email_not_verified", "Email is not verified.", nil)
	case errors.Is(err, authmodel.ErrEmailAlreadyUsed):
		s.writeAPIError(w, r, http.StatusConflict, "email_already_used", "Email is already used.", nil)
	case errors.Is(err, authmodel.ErrCodeInvalid):
		s.writeAPIError(w, r, http.StatusBadRequest, "code_invalid", "Code is invalid or expired.", nil)
	case errors.Is(err, authmodel.ErrCodeCooldown):
		s.writeAPIError(w, r, http.StatusTooManyRequests, "code_cooldown", "Please wait before requesting another code.", nil)
	case errors.Is(err, authmodel.ErrPasswordInvalid):
		s.writeAPIError(w, r, http.StatusBadRequest, "password_invalid", "Password does not meet the required policy.", nil)
	case errors.Is(err, authmodel.ErrPasswordUnchanged):
		s.writeAPIError(w, r, http.StatusBadRequest, "password_unchanged", "New password must differ from the current password.", nil)
	case errors.Is(err, authmodel.ErrProviderUnsupported):
		s.writeAPIError(w, r, http.StatusBadRequest, "provider_unsupported", "Provider is unsupported for this operation.", nil)
	case errors.Is(err, authmodel.ErrOAuthDisabled):
		s.writeAPIError(w, r, http.StatusBadRequest, "provider_disabled", "This sign-in provider is unavailable.", nil)
	case errors.Is(err, authmodel.ErrOAuthEmailUnavailable):
		s.writeAPIError(w, r, http.StatusBadRequest, "oauth_email_unavailable", "GitHub did not provide a usable verified email.", nil)
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
	var req cloudproto.CurrentDeviceReq
	if !s.decodeJSONRequest(w, r, &req) {
		return
	}
	device := Device{Id: req.GetId(), Name: req.GetName(), PublicKey: req.GetPublicKey()}
	if err := s.config.DeviceRepository.UpsertUserDevice(r.Context(), claims.Sub, device); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "Device report is invalid.", err)
		return
	}
	writeJSON(w, http.StatusOK, &cloudproto.CurrentDeviceResp{Accepted: true, Device: s.deviceSummary(device)})
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
	if devices == nil {
		devices = []*cloudproto.DeviceSummary{}
	}
	writeJSON(w, http.StatusOK, &cloudproto.ListDevicesResp{Items: devices})
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
	if isFileGitDeviceRoute(parts) && !s.authorizeFileGitDevice(w, r, deviceId) {
		return
	}
	route := s.routeFor(deviceId)
	endpoint := tunnelRuntimeEndpoint{route: route, deviceId: deviceId, handler: s}
	switch parts[1] {
	case "workspaces":
		s.handleWorkspaceRoute(w, r, endpoint, deviceId, parts)
	case "sessions":
		if len(parts) == 2 && r.Method == http.MethodPost {
			request := &agent.CreateSessionReq{}
			if !s.decodeJSONRequest(w, r, request) {
				return
			}
			s.handleJSONRuntimeWithStatus(w, r, endpoint, deviceId, "create_session", request, "", http.StatusCreated)
			return
		}
		s.writeNotFound(w, r)
	case "shortcuts":
		s.handleShortcutRoute(w, r, endpoint, deviceId, parts[2:])
	default:
		s.writeNotFound(w, r)
	}
}
func (s *Handler) handleShortcutRoute(w http.ResponseWriter, r *http.Request, endpoint runtimeEndpoint, deviceId string, parts []string) {
	if len(parts) == 1 && parts[0] == "order" {
		if r.Method != http.MethodPatch {
			s.methodNotAllowed(w, r, http.MethodPatch)
			return
		}
		request := &agent.UpdateShortcutOrderReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "update_shortcut_order", request, "")
		return
	}
	if len(parts) == 0 {
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
	if len(parts) != 1 || parts[0] == "" {
		s.writeNotFound(w, r)
		return
	}
	shortcutId := parts[0]
	switch r.Method {
	case http.MethodPatch:
		request := &agent.UpdateShortcutReq{}
		if !s.decodeJSONRequest(w, r, request) {
			return
		}
		s.handleJSONRuntime(w, r, endpoint, deviceId, "update_shortcut", &agent.UpdateShortcutRequest{ShortcutId: shortcutId, Request: request}, "")
	case http.MethodDelete:
		s.handleNoContentRuntime(w, r, endpoint, "delete_shortcut", &agent.DeleteShortcutReq{ShortcutId: shortcutId})
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
	if len(parts) >= 4 && parts[3] == "fs" {
		s.handleWorkspaceFsRoute(w, r, endpoint, deviceId, parts[2], parts[4:])
		return
	}
	if len(parts) >= 4 && parts[3] == "scm" {
		s.handleWorkspaceScmRoute(w, r, endpoint, deviceId, parts[2], parts[4:])
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
func (s *Handler) handleDeleteDevice(w http.ResponseWriter, r *http.Request, deviceId string) {
	claims, ok := s.claimsFromRequest(r)
	if !ok {
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
		s.writeRuntimeError(w, r, err)
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
		s.writeRuntimeError(w, r, err)
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
	claims, ok := s.claimsFromRequest(r)
	if !ok {
		s.writeUnauthorized(w, r)
		return
	}
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
	limit, err := s.attachLimitForUser(r.Context(), claims.Sub)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	if err := s.attachQuota.TryAcquire(claims.Sub, limit); err != nil {
		s.writeAPIError(w, r, http.StatusTooManyRequests, quotapkg.CodeAttachExceeded, quotapkg.MessageAttachExceeded, err)
		return
	}
	defer s.attachQuota.Release(claims.Sub)
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{terminalproto.Subprotocol}, OriginPatterns: s.originPatterns(r)})
	if err != nil {
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	cols, rows, hasAttachSize, sizeErr := terminalAttachSizeFromQuery(r)
	if sizeErr != nil {
		s.config.Logger.Warn("terminal attach size invalid", "workspace_id", workspaceId, "session_id", sessionId, "error", sizeErr)
		_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: terminalproto.ErrorCodeBadControl, Message: terminalproto.ErrorMessageBadControl})
		return
	}
	term := &terminalRelay{sessionId: sessionId, browser: conn, done: make(chan struct{}), logger: s.config.Logger}
	route.addTerminal(streamId, term)
	attachReq := &shared.TerminalAttachReq{WorkspaceId: workspaceId, SessionId: sessionId}
	if hasAttachSize {
		attachReq.Cols = int32(cols)
		attachReq.Rows = int32(rows)
	}
	attach := &shared.TunnelFrame{StreamId: streamId, RequestId: s.requestIdFor(w, r), Payload: &shared.TunnelFrame_TerminalAttach{TerminalAttach: attachReq}}
	if err := route.writeFrame(r.Context(), attach); err != nil {
		return
	}
	go func() {
		for {
			messageType, data, err := conn.Read(r.Context())
			if err != nil {
				closeFrame := &shared.TunnelFrame{StreamId: streamId, Payload: &shared.TunnelFrame_Close{Close: &shared.Close{Reason: "browser_disconnected"}}}
				_ = route.writeFrame(context.Background(), closeFrame)
				closeOnce(term.done)
				return
			}
			switch messageType {
			case websocket.MessageText:
				message, err := terminalproto.DecodeClient(data)
				if err != nil {
					s.config.Logger.Warn("terminal control decode failed", "workspace_id", workspaceId, "session_id", sessionId, "error", err)
					_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: terminalproto.ErrorCodeBadControl, Message: terminalproto.ErrorMessageBadControl})
					continue
				}
				switch message.Type {
				case terminalproto.TypeHello:
					continue
				case terminalproto.TypeResize:
					resize := &shared.TunnelFrame{StreamId: streamId, Payload: &shared.TunnelFrame_TerminalResize{TerminalResize: &shared.TerminalResize{Cols: message.Cols, Rows: message.Rows}}}
					_ = route.writeFrame(r.Context(), resize)
				case terminalproto.TypeDetach:
					closeFrame := &shared.TunnelFrame{StreamId: streamId, Payload: &shared.TunnelFrame_Close{Close: &shared.Close{Reason: "browser_detached"}}}
					_ = route.writeFrame(context.Background(), closeFrame)
					closeOnce(term.done)
					return
				case terminalproto.TypePing:
					_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
				}
			case websocket.MessageBinary:
				input := &shared.TunnelFrame{StreamId: streamId, Payload: &shared.TunnelFrame_TerminalInput{TerminalInput: &shared.TerminalInput{Data: data}}}
				_ = route.writeFrame(r.Context(), input)
			}
		}
	}()
	<-term.done
}
func (s *Handler) handleWorkspaceWatchWS(w http.ResponseWriter, r *http.Request, route *agentRoute, workspaceId string) error {
	streamId := "fs-watch-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:   []string{workspacefs.Subprotocol},
		OriginPatterns: s.originPatterns(r),
	})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	watch := &workspaceWatchRelay{
		workspaceId: workspaceId,
		browser:     conn,
		done:        make(chan struct{}),
		logger:      s.config.Logger,
	}
	route.addWorkspaceWatch(streamId, watch)
	defer func() {
		route.removeWorkspaceWatch(streamId)
		closeOnce(watch.done)
		closeFrame := &shared.TunnelFrame{
			StreamId: streamId,
			Payload: &shared.TunnelFrame_Close{
				Close: &shared.Close{Reason: "browser_disconnected"},
			},
		}
		_ = route.writeFrame(context.Background(), closeFrame)
	}()

	subscribe := &shared.TunnelFrame{
		StreamId:  streamId,
		RequestId: s.requestIdFor(w, r),
		Payload: &shared.TunnelFrame_FsWatchSubscribeReq{
			FsWatchSubscribeReq: &agent.FsWatchSubscribeReq{WorkspaceId: workspaceId},
		},
	}
	if err := route.writeFrame(r.Context(), subscribe); err != nil {
		return err
	}

	go func() {
		for {
			_, _, err := conn.Read(r.Context())
			if err != nil {
				closeOnce(watch.done)
				return
			}
		}
	}()
	<-watch.done
	return nil
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
		route.closeWorkspaceWatches("device disconnected")
		s.registry.MarkOffline(hello.GetDeviceId(), time.Now().UTC())
	}()
	ack := &shared.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &shared.TunnelFrame_HelloAck{HelloAck: &shared.HelloAck{ProtocolVersion: tunnel.ProtocolVersion}}}
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
			pong := &shared.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &shared.TunnelFrame_Pong{Pong: &shared.Pong{Nonce: ping.GetNonce()}}}
			_ = route.writeFrame(r.Context(), pong)
		}
	}
}
func (s *Handler) verifyAgentTunnelRequest(r *http.Request) (string, bool) {
	return verifyRepositorySignedTunnelRequest(r.Context(), r, s.config.DeviceRepository, s.configAudience())
}

func (s *Handler) configAudience() string {
	if strings.TrimSpace(s.config.AgentTunnelAudience) != "" {
		return s.config.AgentTunnelAudience
	}
	return "termbridge-cloud"
}

func (s *Handler) devicesForRequest(r *http.Request) ([]*cloudproto.DeviceSummary, error) {
	claims, ok := s.claimsFromRequest(r)
	if !ok || s.config.DeviceRepository == nil {
		return nil, nil
	}
	devices, err := s.config.DeviceRepository.ListDevicesForUser(r.Context(), claims.Sub)
	if err != nil {
		return nil, err
	}
	summaries := make([]*cloudproto.DeviceSummary, 0, len(devices))
	for _, device := range devices {
		summaries = append(summaries, s.deviceSummary(device))
	}
	return summaries, nil
}

func (s *Handler) deviceSummary(device Device) *cloudproto.DeviceSummary {
	if runtimeDevice, ok := s.registry.Get(device.Id); ok {
		if runtimeDevice.Name == "" {
			runtimeDevice.Name = device.Name
		}
		return normalizeDeviceStatus(runtimeDevice)
	}
	return &cloudproto.DeviceSummary{Id: device.Id, Name: device.Name, Online: false, Status: "offline", LastSeen: prototime.FromTime(device.UpdatedAt)}
}

func (s *Handler) disconnectDevice(deviceId string, reason string) {
	s.routeMu.Lock()
	route := s.routes[deviceId]
	delete(s.routes, deviceId)
	s.routeMu.Unlock()
	if route != nil {
		route.closeTerminals(reason)
		route.closeWorkspaceWatches(reason)
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
		old.closeWorkspaceWatches("device reconnected")
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
	config.CloudPublicURL = strings.TrimRight(strings.TrimSpace(config.CloudPublicURL), "/")
	config.Turnstile.SiteKey = strings.TrimSpace(config.Turnstile.SiteKey)
	config.Turnstile.SecretKey = strings.TrimSpace(config.Turnstile.SecretKey)
	config.Turnstile.ExpectedHostname = normalizeHostname(config.Turnstile.ExpectedHostname)
	for index := range config.CloudOAuth.Clients {
		client := &config.CloudOAuth.Clients[index]
		client.ClientId = strings.TrimSpace(client.ClientId)
		client.ClientSecret = strings.TrimSpace(client.ClientSecret)
		client.RedirectUrl = strings.TrimRight(strings.TrimSpace(client.RedirectUrl), "/")
		client.Scopes = cleanCloudOAuthScopes(client.Scopes)
	}
	config.CORSAllowedOrigins = cleanOrigins(config.CORSAllowedOrigins)
	return config
}

func cleanCloudOAuthScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		value := strings.TrimSpace(scope)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
