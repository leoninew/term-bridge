package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentapp "termbridge-go/internal/agent/application/user"
	sharedauth "termbridge-go/internal/shared/common/auth"
)

const registeredCloudOAuthRedirectURL = "http://localhost:9030/cloud/oauth/callback"

func testLocalDevice() agentapp.Device {
	return agentapp.Device{Id: "dev-1", Name: "local-device", PublicKey: "public-key"}
}

func TestCloudOAuthStartReturnsAuthorizeURLInAgentMode(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: "https://cloud.example.test", CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectUrl: registeredCloudOAuthRedirectURL, Scopes: []string{"openid", "email", "profile"}}, LocalDevice: testLocalDevice()})

	request := httptest.NewRequest(http.MethodGet, "/agent-api/cloud-oauth/start", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("cloud oauth start status = %d; body=%s", response.Code, response.Body.String())
	}
	var body struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cloud oauth start response: %v", err)
	}
	authorizeURL, err := url.Parse(body.AuthorizeURL)
	if err != nil {
		t.Fatalf("parse authorize_url = %q: %v", body.AuthorizeURL, err)
	}
	query := authorizeURL.Query()
	if authorizeURL.Scheme != "https" || authorizeURL.Host != "cloud.example.test" || authorizeURL.Path != "/oauth2/authorize" || query.Get("client_id") != "termbridge-local" || query.Get("redirect_uri") != registeredCloudOAuthRedirectURL || query.Get("response_type") != "code" || query.Get("scope") != "openid email profile" || strings.TrimSpace(query.Get("state")) == "" {
		t.Fatalf("authorize_url = %q", body.AuthorizeURL)
	}
	if strings.Contains(body.AuthorizeURL, "dev-1") || strings.Contains(body.AuthorizeURL, "local-device") || strings.Contains(body.AuthorizeURL, "public-key") {
		t.Fatalf("authorize_url leaked local device identity: %q", body.AuthorizeURL)
	}
}

func TestAuthMeReturnsRuntimeLocalCloudSessionSummary(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: "https://cloud.example.test", CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectUrl: registeredCloudOAuthRedirectURL}})
	connectedAt := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	handler.setLocalCloudSession(CloudSessionSummary{GateURL: "https://cloud.example.test", DeviceId: "dev-1", DeviceName: "local-device", ConnectedAt: connectedAt})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent-api/auth/me", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("auth me status = %d; body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Authenticated bool                 `json:"authenticated"`
		CloudSession  *CloudSessionSummary `json:"cloud_session"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode auth me response: %v", err)
	}
	if body.Authenticated {
		t.Fatal("authenticated = true")
	}
	if body.CloudSession == nil {
		t.Fatalf("cloud_session is nil; body=%s", response.Body.String())
	}
	if body.CloudSession.GateURL != "https://cloud.example.test" || body.CloudSession.DeviceId != "dev-1" || body.CloudSession.DeviceName != "local-device" || !body.CloudSession.ConnectedAt.Equal(connectedAt) {
		t.Fatalf("cloud_session = %#v", body.CloudSession)
	}
}

func TestAgentLoginIssuesLocalTokenAndProtectsBusinessRoutes(t *testing.T) {
	authService := NewLocalAuthService(sharedauth.NewTokenService(testJWTKey))
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService})

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/agent-api/workspaces", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated workspaces status = %d, want 401; body=%s", unauthorized.Code, unauthorized.Body.String())
	}

	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/agent-api/auth/login", nil))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var tokenResp TokenResp
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if tokenResp.AccessToken == "" || tokenResp.TokenType != "bearer" {
		t.Fatalf("token response = %#v", tokenResp)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/agent-api/auth/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	meResponse := httptest.NewRecorder()
	handler.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("auth me status = %d, want 200; body=%s", meResponse.Code, meResponse.Body.String())
	}
	var me AuthMeResp
	if err := json.Unmarshal(meResponse.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode auth me response: %v", err)
	}
	if !me.Authenticated || me.User == nil || me.User.Id != "local-agent" || me.User.DisplayName == "" {
		t.Fatalf("auth me = %#v", me)
	}
}

func TestCloudOAuthCallbackPersistsLocalCloudSessionSummaryWithoutTokens(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir, Now: func() time.Time { return time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	var tokenRequest url.Values
	var deviceReport CurrentDeviceReq
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cloud-api/oauth2/token":
			if r.Method != http.MethodPost {
				t.Fatalf("token method = %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse token request form: %v", err)
			}
			tokenRequest = r.PostForm
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"cloud-token","token_type":"bearer"}`))
		case "/cloud-api/devices/current":
			if r.Method != http.MethodPost {
				t.Fatalf("device report method = %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
				t.Fatalf("device report authorization = %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&deviceReport); err != nil {
				t.Fatalf("decode device report: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected cloud request path = %s", r.URL.Path)
		}
	}))
	defer cloud.Close()

	first := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: cloud.URL, CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectUrl: registeredCloudOAuthRedirectURL}, LocalDevice: device, LocalDeviceStateDir: stateDir})
	startResponse := httptest.NewRecorder()
	first.ServeHTTP(startResponse, httptest.NewRequest(http.MethodGet, "/agent-api/cloud-oauth/start", nil))
	if startResponse.Code != http.StatusOK {
		t.Fatalf("start status = %d; body=%s", startResponse.Code, startResponse.Body.String())
	}
	var start CloudOAuthStartResp
	if err := json.Unmarshal(startResponse.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	authorizeURL, err := url.Parse(start.AuthorizeURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	state := authorizeURL.Query().Get("state")
	callbackResponse := httptest.NewRecorder()
	callbackBody := strings.NewReader(`{"code":"auth-code","state":"` + state + `"}`)
	first.ServeHTTP(callbackResponse, httptest.NewRequest(http.MethodPost, "/agent-api/cloud-oauth/callback", callbackBody))
	if callbackResponse.Code != http.StatusOK {
		t.Fatalf("callback status = %d; body=%s", callbackResponse.Code, callbackResponse.Body.String())
	}
	if tokenRequest.Get("grant_type") != "authorization_code" || tokenRequest.Get("code") != "auth-code" || tokenRequest.Get("client_id") != "termbridge-local" || tokenRequest.Get("redirect_uri") != registeredCloudOAuthRedirectURL {
		t.Fatalf("token request = %#v", tokenRequest)
	}
	if deviceReport.Id != device.Id || deviceReport.Name != device.Name || deviceReport.PublicKey != device.PublicKey {
		t.Fatalf("device report = %#v", deviceReport)
	}

	second := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: cloud.URL, CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectUrl: registeredCloudOAuthRedirectURL}, LocalDevice: agentapp.Device{Id: device.Id, Name: device.Name, PublicKey: device.PublicKey}, LocalDeviceStateDir: stateDir})
	response := httptest.NewRecorder()
	second.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent-api/auth/me", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("auth me status = %d; body=%s", response.Code, response.Body.String())
	}
	var body struct {
		CloudSession *CloudSessionSummary `json:"cloud_session"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode auth me response: %v", err)
	}
	if body.CloudSession == nil || body.CloudSession.GateURL != cloud.URL || body.CloudSession.DeviceId != device.Id || body.CloudSession.DeviceName != device.Name || body.CloudSession.ConnectedAt.IsZero() {
		t.Fatalf("cloud_session = %#v", body.CloudSession)
	}
	data, err := os.ReadFile(filepath.Join(stateDir, agentapp.DeviceIdentityFileName))
	if err != nil {
		t.Fatalf("read device identity file error = %v", err)
	}
	serialized := strings.ToLower(string(data))
	if strings.Contains(serialized, "access_token") || strings.Contains(serialized, "refresh_token") || strings.Contains(serialized, "cloud-token") || strings.Contains(serialized, "bearer") {
		t.Fatalf("device identity persisted token material: %s", string(data))
	}
}

func TestCloudOAuthCallbackRejectsInvalidAndExpiredStateBeforeCloudExchange(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("cloud endpoint should not be called for invalid state")
	}))
	defer cloud.Close()
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: cloud.URL, CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectUrl: registeredCloudOAuthRedirectURL}, LocalDevice: testLocalDevice()})

	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "missing state", body: `{"code":"auth-code"}`, wantStatus: http.StatusBadRequest},
		{name: "missing code", body: `{"state":"state-1"}`, wantStatus: http.StatusBadRequest},
		{name: "unknown state", body: `{"code":"auth-code","state":"missing"}`, wantStatus: http.StatusUnauthorized},
		{name: "expired state", body: `{"code":"auth-code","state":"expired"}`, wantStatus: http.StatusUnauthorized},
	}
	handler.storeCloudOAuthState(CloudOAuthState{Value: "expired", ExpiresAt: time.Now().Add(-time.Second)}, time.Now())
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/agent-api/cloud-oauth/callback", strings.NewReader(tc.body)))
			if response.Code != tc.wantStatus {
				t.Fatalf("callback status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
		})
	}
}

func TestCloudOAuthCallbackConsumesStateOnce(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	cloudCalls := 0
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cloudCalls++
		switch r.URL.Path {
		case "/cloud-api/oauth2/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"cloud-token","token_type":"bearer"}`))
		case "/cloud-api/devices/current":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected cloud request path = %s", r.URL.Path)
		}
	}))
	defer cloud.Close()
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: cloud.URL, CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectUrl: registeredCloudOAuthRedirectURL}, LocalDevice: device, LocalDeviceStateDir: stateDir})
	handler.storeCloudOAuthState(CloudOAuthState{Value: "state-1", ExpiresAt: time.Now().Add(time.Minute)}, time.Now())

	body := `{"code":"auth-code","state":"state-1"}`
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/agent-api/cloud-oauth/callback", strings.NewReader(body)))
	if first.Code != http.StatusOK {
		t.Fatalf("first callback status = %d; body=%s", first.Code, first.Body.String())
	}
	callsAfterFirst := cloudCalls
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/agent-api/cloud-oauth/callback", strings.NewReader(body)))
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("second callback status = %d, want 401; body=%s", second.Code, second.Body.String())
	}
	if cloudCalls != callsAfterFirst {
		t.Fatalf("cloud calls after reused state = %d, want %d", cloudCalls, callsAfterFirst)
	}
}
