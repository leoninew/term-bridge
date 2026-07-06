package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentapp "termbridge/internal/agent/application/user"
	sharedauth "termbridge/internal/shared/common/auth"
)

func testLocalDevice() agentapp.Device {
	return agentapp.Device{Id: "dev-1", Name: "local-device", PublicKey: "public-key"}
}

func TestAuthMeReturnsRuntimeLocalCloudSessionSummary(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: "https://cloud.example.test"})
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

func TestCloudConnectReportsCurrentDeviceWithCloudToken(t *testing.T) {
	authService := NewLocalAuthService(sharedauth.NewTokenService(testJWTKey))
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	var deviceReport CurrentDeviceReq
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cloud-api/devices/current" {
			t.Fatalf("unexpected cloud request path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("device report authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&deviceReport); err != nil {
			t.Fatalf("decode device report: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer cloud.Close()
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: cloud.URL, LocalDevice: device, LocalDeviceStateDir: stateDir})
	agentToken := agentToken(t, handler)
	request := httptest.NewRequest(http.MethodPost, "/agent-api/cloud/connect", strings.NewReader(`{"cloud_token":"cloud-token"}`))
	request.Header.Set("Authorization", "Bearer "+agentToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("cloud connect status = %d; body=%s", response.Code, response.Body.String())
	}
	if deviceReport.Id != device.Id || deviceReport.Name != device.Name || deviceReport.PublicKey != device.PublicKey {
		t.Fatalf("device report = %#v", deviceReport)
	}
	var body CloudConnectResp
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cloud connect response: %v", err)
	}
	if body.CloudSession.GateURL != cloud.URL || body.CloudSession.DeviceId != device.Id || body.CloudSession.DeviceName != device.Name {
		t.Fatalf("cloud session = %#v", body.CloudSession)
	}
}

func TestCloudConnectPersistsLocalCloudSessionSummaryWithoutTokens(t *testing.T) {
	authService := NewLocalAuthService(sharedauth.NewTokenService(testJWTKey))
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir, Now: func() time.Time { return time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	var deviceReport CurrentDeviceReq
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cloud-api/devices/current" {
			t.Fatalf("unexpected cloud request path = %s", r.URL.Path)
		}
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
	}))
	defer cloud.Close()

	first := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: cloud.URL, LocalDevice: device, LocalDeviceStateDir: stateDir})
	agentToken := agentToken(t, first)
	connectRequest := httptest.NewRequest(http.MethodPost, "/agent-api/cloud/connect", strings.NewReader(`{"cloud_token":"cloud-token"}`))
	connectRequest.Header.Set("Authorization", "Bearer "+agentToken)
	connectResponse := httptest.NewRecorder()
	first.ServeHTTP(connectResponse, connectRequest)
	if connectResponse.Code != http.StatusOK {
		t.Fatalf("cloud connect status = %d; body=%s", connectResponse.Code, connectResponse.Body.String())
	}
	if deviceReport.Id != device.Id || deviceReport.Name != device.Name || deviceReport.PublicKey != device.PublicKey {
		t.Fatalf("device report = %#v", deviceReport)
	}

	second := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: cloud.URL, LocalDevice: agentapp.Device{Id: device.Id, Name: device.Name, PublicKey: device.PublicKey}, LocalDeviceStateDir: stateDir})
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

func TestCloudConnectRequiresCloudTokenBeforeDeviceReport(t *testing.T) {
	authService := NewLocalAuthService(sharedauth.NewTokenService(testJWTKey))
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("cloud endpoint should not be called without cloud token")
	}))
	defer cloud.Close()
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudGateURL: cloud.URL, LocalDevice: testLocalDevice()})
	agentToken := agentToken(t, handler)

	request := httptest.NewRequest(http.MethodPost, "/agent-api/cloud/connect", strings.NewReader(`{"cloud_token":" "}`))
	request.Header.Set("Authorization", "Bearer "+agentToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("cloud connect status = %d, want 400; body=%s", response.Code, response.Body.String())
	}
}

func agentToken(t *testing.T, handler *Handler) string {
	t.Helper()
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/agent-api/auth/login", nil))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d; body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var token TokenResp
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &token); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if token.AccessToken == "" {
		t.Fatalf("login token is empty: %#v", token)
	}
	return token.AccessToken
}
