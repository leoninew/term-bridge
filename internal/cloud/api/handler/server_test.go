package api

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
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	authmodel "gitee.com/leoninew/TermBridge-go/internal/cloud/model/user/auth"
	cloud "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

type testErrorResponse struct {
	Code      string `json:"code"`
	Error     string `json:"error"`
	RequestId string `json:"request_id"`
}

type acceptingTurnstileVerifier struct{}

func (acceptingTurnstileVerifier) Verify(context.Context, string, string) error {
	return nil
}

func testAuthSecurityConfig() (TurnstileConfig, CSRFConfig) {
	return TurnstileConfig{Enabled: true, Verify: acceptingTurnstileVerifier{}}, CSRFConfig{Tokens: NewCSRFTokens(0, 0)}
}

func testLoginRequestBody(t *testing.T, handler *Handler, username string, password string) string {
	t.Helper()
	csrfToken, err := handler.config.CSRF.Tokens.Issue()
	if err != nil {
		t.Fatalf("issue CSRF token: %v", err)
	}
	return fmt.Sprintf(`{"username":%q,"password":%q,"turnstile_token":"turnstile-test-token","csrf_token":%q}`, username, password, csrfToken)
}

func testCloudConfig() Config {
	turnstile, csrf := testAuthSecurityConfig()
	return Config{Logger: slog.Default(), AuthService: newTestAuthService(sharedauth.NewTokenService(testJWTKey)), DeviceRepository: testDeviceRepository{}, Turnstile: turnstile, CSRF: csrf}
}

var testTunnelPrivateKey = ed25519.NewKeyFromSeed([]byte("12345678901234567890123456789012"))

type testDeviceRepository struct{}

func (testDeviceRepository) UpsertDeviceBinding(context.Context, string, Device) error { return nil }
func (testDeviceRepository) UpsertUserDevice(context.Context, string, Device) error    { return nil }
func (testDeviceRepository) UserOwnsDevice(context.Context, string, string) (bool, error) {
	return true, nil
}
func (testDeviceRepository) DeleteUserDevice(context.Context, string, string) (bool, error) {
	return true, nil
}
func (testDeviceRepository) PublicKey(_ context.Context, deviceId string) (string, error) {
	if deviceId != "0f490dee643b01b06e0ea84c253a9005" {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(testTunnelPrivateKey.Public().(ed25519.PublicKey)), nil
}
func (testDeviceRepository) ListDevicesForUser(context.Context, string) ([]Device, error) {
	return nil, nil
}

func signedTestTunnelHeader(t *testing.T) http.Header {
	t.Helper()
	header, err := tunnel.SignedTunnelHeader(http.MethodGet, "/api/agent/tunnel", "termbridge-cloud", "0f490dee643b01b06e0ea84c253a9005", testTunnelPrivateKey, time.Now(), "test-nonce")
	if err != nil {
		t.Fatalf("SignedTunnelHeader() error = %v", err)
	}
	return header
}

func TestHealth(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestAuthEndpoints(t *testing.T) {
	server := New(testCloudConfig())
	devicesResponse := httptest.NewRecorder()
	server.ServeHTTP(devicesResponse, httptest.NewRequest(http.MethodGet, "/api/devices", nil))
	if devicesResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated devices status = %d, want 401", devicesResponse.Code)
	}
	assertAPIError(t, devicesResponse, http.StatusUnauthorized, errorCodeUnauthorized)

	loginResponse := httptest.NewRecorder()
	server.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(testLoginRequestBody(t, server, "admin", "admin"))))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if tokenResp.AccessToken == "" {
		t.Fatal("access_token is empty")
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	meResponse := httptest.NewRecorder()
	server.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("me status = %d, want 200; body=%s", meResponse.Code, meResponse.Body.String())
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	logoutResponse := httptest.NewRecorder()
	server.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", logoutResponse.Code)
	}

	devicesRequestAfterExpired := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	devicesRequestAfterExpired.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	devicesResponseAfterExpired := httptest.NewRecorder()
	server.ServeHTTP(devicesResponseAfterExpired, devicesRequestAfterExpired)
	if devicesResponseAfterExpired.Code != http.StatusOK {
		t.Fatalf("devices with valid token status = %d, want 200", devicesResponseAfterExpired.Code)
	}
	var devicesBody cloud.ListDevicesResp
	if err := codec.UnmarshalProtoJSON(devicesResponseAfterExpired.Body.Bytes(), &devicesBody); err != nil {
		t.Fatalf("decode devices response: %v; body=%s", err, devicesResponseAfterExpired.Body.String())
	}
	if len(devicesBody.GetItems()) != 0 {
		t.Fatalf("devices count = %d, want 0", len(devicesBody.GetItems()))
	}
}

func TestExternalOAuthInitiationRoutes(t *testing.T) {
	server := New(Config{
		Logger:      slog.Default(),
		AuthService: newTestAuthService(sharedauth.NewTokenService(testJWTKey)),
	})

	for _, providerId := range []string{"google", "github"} {
		t.Run(providerId, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/oauth2/"+providerId, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
			}
			var body struct {
				AuthUrl string `json:"auth_url"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.AuthUrl != "https://auth.example/"+providerId {
				t.Fatalf("auth_url = %q, want provider URL", body.AuthUrl)
			}
		})
	}

	wrongMethodResponse := httptest.NewRecorder()
	server.ServeHTTP(wrongMethodResponse, httptest.NewRequest(http.MethodPost, "/api/oauth2/google", nil))
	if wrongMethodResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method status = %d, want 405", wrongMethodResponse.Code)
	}

	callbackResponse := httptest.NewRecorder()
	server.ServeHTTP(callbackResponse, httptest.NewRequest(http.MethodPost, "/api/oauth2/google/callback", nil))
	if callbackResponse.Code != http.StatusBadRequest {
		t.Fatalf("callback status = %d, want 400", callbackResponse.Code)
	}

	oldRouteResponse := httptest.NewRecorder()
	server.ServeHTTP(oldRouteResponse, httptest.NewRequest(http.MethodGet, "/api/auth/oauth/google", nil))
	if oldRouteResponse.Code != http.StatusNotFound {
		t.Fatalf("old initiation route status = %d, want 404", oldRouteResponse.Code)
	}

	oldCallbackRouteResponse := httptest.NewRecorder()
	server.ServeHTTP(oldCallbackRouteResponse, httptest.NewRequest(http.MethodPost, "/api/auth/oauth/google/callback", nil))
	if oldCallbackRouteResponse.Code != http.StatusNotFound {
		t.Fatalf("old callback route status = %d, want 404", oldCallbackRouteResponse.Code)
	}
}

func TestOAuthAuthorizationCodeCanBeExchangedOnce(t *testing.T) {
	tokens := sharedauth.NewTokenService(testJWTKey)
	server := New(Config{
		Logger:      slog.Default(),
		AuthService: newTestAuthService(tokens),
		CloudOAuth: CloudOAuthConfig{
			Clients: []CloudOAuthClientConfig{{
				ClientId:     "termbridge-agent",
				ClientSecret: "agent-secret",
				RedirectUrl:  "http://127.0.0.1:9033/oauth/callback",
				Scopes:       []string{"openid", "email", "profile"},
			}},
		},
	})
	userToken, err := tokens.Sign(sharedauth.Claims{Sub: "user-1", Email: "user-1@example.test", Provider: "email"})
	if err != nil {
		t.Fatalf("sign user token: %v", err)
	}
	callbackUrl := "http://127.0.0.1:9033/oauth/callback"
	authorizeValues := url.Values{}
	authorizeValues.Set("response_type", "code")
	authorizeValues.Set("client_id", "termbridge-agent")
	authorizeValues.Set("redirect_uri", callbackUrl)
	authorizeValues.Set("state", "state-1")
	authorizeRequest := httptest.NewRequest(http.MethodPost, "/api/oauth2/authorize?"+authorizeValues.Encode(), nil)
	authorizeRequest.Header.Set("Authorization", "Bearer "+userToken)
	authorizeResponse := httptest.NewRecorder()
	server.ServeHTTP(authorizeResponse, authorizeRequest)
	if authorizeResponse.Code != http.StatusOK {
		t.Fatalf("authorize status = %d, want 200; body=%s", authorizeResponse.Code, authorizeResponse.Body.String())
	}
	var authorizeBody struct {
		RedirectUrl string `json:"redirect_url"`
	}
	if err := json.Unmarshal(authorizeResponse.Body.Bytes(), &authorizeBody); err != nil {
		t.Fatalf("decode authorize response: %v", err)
	}
	callback, err := url.Parse(authorizeBody.RedirectUrl)
	if err != nil {
		t.Fatalf("parse callback redirect: %v", err)
	}
	if callback.Query().Get("state") != "state-1" {
		t.Fatalf("callback state = %q, want original state", callback.Query().Get("state"))
	}
	code := callback.Query().Get("code")
	if code == "" {
		t.Fatal("authorization code is empty")
	}
	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("client_id", "termbridge-agent")
	tokenForm.Set("client_secret", "agent-secret")
	tokenForm.Set("redirect_uri", callbackUrl)
	tokenForm.Set("code", code)
	tokenRequest := httptest.NewRequest(http.MethodPost, "/api/oauth2/token", strings.NewReader(tokenForm.Encode()))
	tokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResponse := httptest.NewRecorder()
	server.ServeHTTP(tokenResponse, tokenRequest)
	if tokenResponse.Code != http.StatusOK {
		t.Fatalf("token status = %d, want 200; body=%s", tokenResponse.Code, tokenResponse.Body.String())
	}
	var tokenBody struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(tokenResponse.Body.Bytes(), &tokenBody); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if tokenBody.AccessToken == "" {
		t.Fatal("access token is empty")
	}

	replayRequest := httptest.NewRequest(http.MethodPost, "/api/oauth2/token", strings.NewReader(tokenForm.Encode()))
	replayRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	replayResponse := httptest.NewRecorder()
	server.ServeHTTP(replayResponse, replayRequest)
	assertAPIError(t, replayResponse, http.StatusBadRequest, errorCodeBadRequest)
}

func TestLoginRejectsInvalidJSON(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username"`)))
	assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
}

func TestCloudAuthEndpointsReturnStructuredErrors(t *testing.T) {
	service := newTestAuthService(sharedauth.NewTokenService(testJWTKey)).(*testAuthService)
	service.loginErr = authmodel.ErrEmailNotVerified
	service.registerErr = authmodel.ErrEmailAlreadyUsed
	service.verifyEmailErr = authmodel.ErrCodeInvalid
	service.resendVerificationErr = authmodel.ErrCodeCooldown
	service.changePasswordErr = authmodel.ErrPasswordInvalid
	service.confirmPasswordResetErr = authmodel.ErrCodeInvalid
	turnstile, csrf := testAuthSecurityConfig()
	server := New(Config{Logger: slog.Default(), AuthService: service, Turnstile: turnstile, CSRF: csrf})
	token, err := service.tokens.Sign(sharedauth.Claims{Sub: "user-1", Email: "user@example.test", Provider: "email"})
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		body       string
		status     int
		code       string
		authorized bool
	}{
		{name: "login", path: "/api/auth/login", body: testLoginRequestBody(t, server, "user@example.test", "password"), status: http.StatusForbidden, code: "email_not_verified"},
		{name: "register", path: "/api/auth/register", body: `{"email":"user@example.test","password":"password","turnstile_token":"turnstile-test-token"}`, status: http.StatusConflict, code: "email_already_used"},
		{name: "verify email", path: "/api/auth/email/verify", body: `{"email":"user@example.test","code":"ABC123"}`, status: http.StatusBadRequest, code: "code_invalid"},
		{name: "resend verification", path: "/api/auth/email/verification/resend", body: `{"email":"user@example.test"}`, status: http.StatusTooManyRequests, code: "code_cooldown"},
		{name: "change password", path: "/api/auth/password/change", body: `{"current_password":"current-password","new_password":"short"}`, status: http.StatusBadRequest, code: "password_invalid", authorized: true},
		{name: "confirm password reset", path: "/api/auth/password-reset/confirm", body: `{"email":"user@example.test","code":"ABC123","new_password":"password"}`, status: http.StatusBadRequest, code: "code_invalid"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, bytes.NewBufferString(test.body))
			request.Header.Set(requestIdHeader, "req_auth_contract")
			if test.authorized {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			body := assertAPIError(t, response, test.status, test.code)
			if body.RequestId != "req_auth_contract" {
				t.Fatalf("request_id = %q, want preserved inbound value", body.RequestId)
			}
		})
	}
}

func TestCloudAuthEndpointsRejectWrongMethodWithStructuredErrors(t *testing.T) {
	service := newTestAuthService(sharedauth.NewTokenService(testJWTKey)).(*testAuthService)
	turnstile, csrf := testAuthSecurityConfig()
	server := New(Config{Logger: slog.Default(), AuthService: service, Turnstile: turnstile, CSRF: csrf})
	token, err := service.tokens.Sign(sharedauth.Claims{Sub: "user-1", Email: "user@example.test", Provider: "email"})
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	for _, test := range []struct {
		path       string
		authorized bool
	}{
		{path: "/api/auth/login"},
		{path: "/api/auth/register"},
		{path: "/api/auth/email/verify"},
		{path: "/api/auth/email/verification/resend"},
		{path: "/api/auth/password/change", authorized: true},
		{path: "/api/auth/password-reset/request"},
		{path: "/api/auth/password-reset/confirm"},
	} {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.authorized {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			assertAPIError(t, response, http.StatusMethodNotAllowed, errorCodeMethodNotAllowed)
		})
	}
}

func TestCloudAuthEndpointsRejectMalformedJSONWithStructuredErrors(t *testing.T) {
	turnstile, csrf := testAuthSecurityConfig()
	server := New(Config{Logger: slog.Default(), AuthService: newTestAuthService(sharedauth.NewTokenService(testJWTKey)), Turnstile: turnstile, CSRF: csrf})

	for _, path := range []string{
		"/api/auth/login",
		"/api/auth/register",
		"/api/auth/email/verify",
		"/api/auth/email/verification/resend",
		"/api/auth/password-reset/request",
		"/api/auth/password-reset/confirm",
	} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"email"`)))
			assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
		})
	}
}

func TestPasswordResetRequestSuppressesServiceOutcomes(t *testing.T) {
	service := newTestAuthService(sharedauth.NewTokenService(testJWTKey)).(*testAuthService)
	server := New(Config{Logger: slog.Default(), AuthService: service})

	outcomes := []error{nil, errors.New("smtp credential secret failed")}
	for _, outcome := range outcomes {
		service.requestPasswordResetErr = outcome
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/password-reset/request", bytes.NewBufferString(`{"email":"user@example.test"}`)))
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204; body=%s", response.Code, response.Body.String())
		}
		if response.Body.Len() != 0 {
			t.Fatalf("body = %q, want empty", response.Body.String())
		}
		if response.Header().Get("Content-Type") != "" {
			t.Fatalf("Content-Type = %q, want empty", response.Header().Get("Content-Type"))
		}
	}
}

func TestCloudAuthUnknownCausesDoNotLeak(t *testing.T) {
	service := newTestAuthService(sharedauth.NewTokenService(testJWTKey)).(*testAuthService)
	service.registerErr = errors.New("database password=super-secret failed")
	turnstile, csrf := testAuthSecurityConfig()
	server := New(Config{Logger: slog.Default(), AuthService: service, Turnstile: turnstile, CSRF: csrf})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"user@example.test","password":"password","turnstile_token":"turnstile-test-token"}`)))
	body := assertAPIError(t, response, http.StatusInternalServerError, errorCodeInternal)
	if body.Error != errorMessageInternal {
		t.Fatalf("error = %q, want generic internal message", body.Error)
	}
	if strings.Contains(response.Body.String(), "super-secret") {
		t.Fatalf("response leaked service cause: %s", response.Body.String())
	}
}

func TestLegacyCloudApiPathIsRejected(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/cloud-api/health", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("legacy cloud health status = %d, want 404", response.Code)
	}
}

func TestHealthRejectsUnsupportedMethod(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/health", nil))
	assertAPIError(t, response, http.StatusMethodNotAllowed, errorCodeMethodNotAllowed)
}

func TestUnknownAPIPathReturnsStructuredNotFound(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	assertAPIError(t, response, http.StatusNotFound, errorCodeNotFound)
}

func assertAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) testErrorResponse {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	var body testErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, response.Body.String())
	}
	if body.Code != code {
		t.Fatalf("code = %q, want %q; body=%s", body.Code, code, response.Body.String())
	}
	if body.Error == "" {
		t.Fatalf("error is empty; body=%s", response.Body.String())
	}
	if body.RequestId == "" {
		t.Fatalf("requestId is empty; body=%s", response.Body.String())
	}
	if headerRequestId := response.Header().Get(requestIdHeader); headerRequestId != body.RequestId {
		t.Fatalf("X-Request-ID = %q, want envelope request_id %q", headerRequestId, body.RequestId)
	}
	var raw map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw error response: %v", err)
	}
	if _, ok := raw["message"]; ok {
		t.Fatalf("error response must not include legacy message field: %s", response.Body.String())
	}
	return body
}
