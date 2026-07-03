package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentapp "termbridge-go/internal/agent/application"
	sharedauth "termbridge-go/internal/shared/auth"
)

const registeredCloudOAuthRedirectURL = "http://localhost:9030/cloud/oauth/callback"

func testLocalDevice() agentapp.Device {
	return agentapp.Device{Id: "dev-1", Name: "local-device"}
}

func TestCloudOAuthStartReturnsAuthorizeURLInLocalMode(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudOAuthAttemptStore: newTestCloudOAuthAttemptStore(t.TempDir()), CloudGateURL: "https://cloud.example.test", CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectURL: registeredCloudOAuthRedirectURL, Scopes: []string{"openid", "email", "profile"}}, LocalDevice: testLocalDevice()})

	request := httptest.NewRequest(http.MethodGet, "/api/cloud-oauth/start", nil)
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
	if !strings.HasPrefix(body.AuthorizeURL, "https://cloud.example.test/oauth2/authorize?") || !strings.Contains(body.AuthorizeURL, "client_id=termbridge-local") || !strings.Contains(body.AuthorizeURL, "redirect_uri=http%3A%2F%2Flocalhost%3A9030%2Fcloud%2Foauth%2Fcallback") || !strings.Contains(body.AuthorizeURL, "response_type=code") || !strings.Contains(body.AuthorizeURL, "scope=openid+email+profile") || !strings.Contains(body.AuthorizeURL, "state=") {
		t.Fatalf("authorize_url = %q", body.AuthorizeURL)
	}
}

func TestAuthMeReturnsRuntimeLocalCloudSessionSummary(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudOAuthAttemptStore: newTestCloudOAuthAttemptStore(t.TempDir()), CloudGateURL: "https://cloud.example.test", CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectURL: registeredCloudOAuthRedirectURL}})
	connectedAt := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	handler.setLocalCloudSession(CloudSessionSummary{GateURL: "https://cloud.example.test", DeviceId: "dev-1", DeviceName: "local-device", ConnectedAt: connectedAt})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
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

func TestAuthMeDoesNotPersistLocalCloudSessionAcrossHandlerRestart(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	stateDir := t.TempDir()
	first := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudOAuthAttemptStore: newTestCloudOAuthAttemptStore(stateDir), CloudGateURL: "https://cloud.example.test", CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectURL: registeredCloudOAuthRedirectURL}})
	first.setLocalCloudSession(CloudSessionSummary{GateURL: "https://cloud.example.test", DeviceId: "dev-1", DeviceName: "local-device", ConnectedAt: time.Now().UTC()})

	second := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudOAuthAttemptStore: newTestCloudOAuthAttemptStore(stateDir), CloudGateURL: "https://cloud.example.test", CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectURL: registeredCloudOAuthRedirectURL}})
	response := httptest.NewRecorder()
	second.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("auth me status = %d; body=%s", response.Code, response.Body.String())
	}
	var body struct {
		CloudSession *CloudSessionSummary `json:"cloud_session"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode auth me response: %v", err)
	}
	if body.CloudSession != nil {
		t.Fatalf("cloud_session = %#v, want nil after handler restart", body.CloudSession)
	}
}
