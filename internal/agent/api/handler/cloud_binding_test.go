package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	cloudv1 "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/prototime"

	"google.golang.org/protobuf/proto"
)

func testLocalDevice() agentapp.Device {
	return agentapp.Device{Id: "dev-1", Name: "local-device", PublicKey: "public-key"}
}

func TestAuthMeReturnsRuntimeLocalCloudSessionSummary(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudPublicURL: "https://cloud.example.test"})
	connectedAt := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	handler.setLocalCloudSession(&cloudv1.CloudSessionSummary{PublicUrl: "https://cloud.example.test", DeviceId: "dev-1", DeviceName: "local-device", ConnectedAt: prototime.FromTime(connectedAt)})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/local-api/auth/me", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("auth me status = %d; body=%s", response.Code, response.Body.String())
	}
	var body cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode auth me response: %v", err)
	}
	if body.Authenticated {
		t.Fatal("authenticated = true")
	}
	if body.CloudSession == nil {
		t.Fatalf("cloud_session is nil; body=%s", response.Body.String())
	}
	if body.CloudSession.GetPublicUrl() != "https://cloud.example.test" || body.CloudSession.GetDeviceId() != "dev-1" || body.CloudSession.GetDeviceName() != "local-device" || !prototime.ToTime(body.CloudSession.GetConnectedAt()).Equal(connectedAt) {
		t.Fatalf("cloud_session = %#v", body.CloudSession)
	}
}

func TestAuthMeReturnsLocalDeviceSummary(t *testing.T) {
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	device := testLocalDevice()
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, LocalDevice: device})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/local-api/auth/me", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("auth me status = %d; body=%s", response.Code, response.Body.String())
	}
	var body cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode auth me response: %v", err)
	}
	if body.GetDevice().GetId() != device.Id || body.GetDevice().GetName() != device.Name {
		t.Fatalf("device id=%q name=%q", body.GetDevice().GetId(), body.GetDevice().GetName())
	}
}

func TestAgentLoginIssuesLocalTokenAndProtectsBusinessRoutes(t *testing.T) {
	authService := NewLocalAuthService(sharedauth.NewTokenService(testJWTKey))
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService})

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/local-api/workspaces", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated workspaces status = %d, want 401; body=%s", unauthorized.Code, unauthorized.Body.String())
	}

	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/local-api/auth/login", nil))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var tokenResp cloudv1.TokenResp
	if err := codec.UnmarshalProtoJSON(loginResponse.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if tokenResp.AccessToken == "" || tokenResp.TokenType != "bearer" {
		t.Fatalf("token response access_token=%q token_type=%q", tokenResp.GetAccessToken(), tokenResp.GetTokenType())
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/local-api/auth/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	meResponse := httptest.NewRecorder()
	handler.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("auth me status = %d, want 200; body=%s", meResponse.Code, meResponse.Body.String())
	}
	var me cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(meResponse.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode auth me response: %v", err)
	}
	if !me.GetAuthenticated() || me.GetUser() == nil || me.GetUser().GetId() != "local-agent" || me.GetUser().GetDisplayName() == "" {
		t.Fatalf("auth me authenticated=%v user_id=%q display_name=%q", me.GetAuthenticated(), me.GetUser().GetId(), me.GetUser().GetDisplayName())
	}
}

func TestCloudConnectReportsCurrentDeviceWithCloudToken(t *testing.T) {
	authService := NewLocalAuthService(sharedauth.NewTokenService(testJWTKey))
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	var deviceReport cloudv1.CurrentDeviceReq
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cloud-api/devices/current" {
			t.Fatalf("unexpected cloud request path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("device report authorization = %q", got)
		}
		if err := decodeProtoJSONBody(r, &deviceReport); err != nil {
			t.Fatalf("decode device report: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer cloud.Close()
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudPublicURL: cloud.URL, LocalDevice: device, LocalDeviceStateDir: stateDir})
	localToken := localToken(t, handler)
	request := httptest.NewRequest(http.MethodPost, "/local-api/cloud/connect", strings.NewReader(`{"cloud_token":"cloud-token"}`))
	request.Header.Set("Authorization", "Bearer "+localToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("cloud connect status = %d; body=%s", response.Code, response.Body.String())
	}
	if deviceReport.GetId() != device.Id || deviceReport.GetName() != device.Name || deviceReport.GetPublicKey() != device.PublicKey {
		t.Fatalf("device report id=%q name=%q public_key=%q", deviceReport.GetId(), deviceReport.GetName(), deviceReport.GetPublicKey())
	}
	var body cloudv1.CloudConnectResp
	if err := codec.UnmarshalProtoJSON(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cloud connect response: %v", err)
	}
	if body.CloudSession.GetPublicUrl() != cloud.URL || body.CloudSession.GetDeviceId() != device.Id || body.CloudSession.GetDeviceName() != device.Name {
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
	var deviceReport cloudv1.CurrentDeviceReq
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
		if err := decodeProtoJSONBody(r, &deviceReport); err != nil {
			t.Fatalf("decode device report: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer cloud.Close()

	first := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudPublicURL: cloud.URL, LocalDevice: device, LocalDeviceStateDir: stateDir})
	localToken := localToken(t, first)
	connectRequest := httptest.NewRequest(http.MethodPost, "/local-api/cloud/connect", strings.NewReader(`{"cloud_token":"cloud-token"}`))
	connectRequest.Header.Set("Authorization", "Bearer "+localToken)
	connectResponse := httptest.NewRecorder()
	first.ServeHTTP(connectResponse, connectRequest)
	if connectResponse.Code != http.StatusOK {
		t.Fatalf("cloud connect status = %d; body=%s", connectResponse.Code, connectResponse.Body.String())
	}
	if deviceReport.GetId() != device.Id || deviceReport.GetName() != device.Name || deviceReport.GetPublicKey() != device.PublicKey {
		t.Fatalf("device report id=%q name=%q public_key=%q", deviceReport.GetId(), deviceReport.GetName(), deviceReport.GetPublicKey())
	}

	second := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudPublicURL: cloud.URL, LocalDevice: agentapp.Device{Id: device.Id, Name: device.Name, PublicKey: device.PublicKey}, LocalDeviceStateDir: stateDir})
	response := httptest.NewRecorder()
	second.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/local-api/auth/me", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("auth me status = %d; body=%s", response.Code, response.Body.String())
	}
	var body cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode auth me response: %v", err)
	}
	if body.CloudSession == nil || body.CloudSession.GetPublicUrl() != cloud.URL || body.CloudSession.GetDeviceId() != device.Id || body.CloudSession.GetDeviceName() != device.Name || body.CloudSession.GetConnectedAt() == nil {
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

func TestCloudDisconnectClearsLocalCloudSessionWithoutUnbindingCloudDevice(t *testing.T) {
	authService := NewLocalAuthService(sharedauth.NewTokenService(testJWTKey))
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir, Now: func() time.Time { return time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	var cloudRequestPaths []string
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cloudRequestPaths = append(cloudRequestPaths, r.Method+" "+r.URL.Path)
		if r.URL.Path != "/cloud-api/devices/current" || r.Method != http.MethodPost {
			t.Fatalf("unexpected cloud request = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer cloud.Close()
	var sessionEvents []*cloudv1.CloudSessionSummary
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudPublicURL: cloud.URL, LocalDevice: device, LocalDeviceStateDir: stateDir, OnLocalCloudSession: func(summary *cloudv1.CloudSessionSummary) {
		if summary == nil {
			sessionEvents = append(sessionEvents, nil)
			return
		}
		sessionEvents = append(sessionEvents, proto.Clone(summary).(*cloudv1.CloudSessionSummary))
	}})
	localToken := localToken(t, handler)
	connectRequest := httptest.NewRequest(http.MethodPost, "/local-api/cloud/connect", strings.NewReader(`{"cloud_token":"cloud-token"}`))
	connectRequest.Header.Set("Authorization", "Bearer "+localToken)
	connectResponse := httptest.NewRecorder()
	handler.ServeHTTP(connectResponse, connectRequest)
	if connectResponse.Code != http.StatusOK {
		t.Fatalf("cloud connect status = %d; body=%s", connectResponse.Code, connectResponse.Body.String())
	}

	disconnectRequest := httptest.NewRequest(http.MethodPost, "/local-api/cloud/disconnect", nil)
	disconnectRequest.Header.Set("Authorization", "Bearer "+localToken)
	disconnectResponse := httptest.NewRecorder()
	handler.ServeHTTP(disconnectResponse, disconnectRequest)
	if disconnectResponse.Code != http.StatusNoContent {
		t.Fatalf("cloud disconnect status = %d; body=%s", disconnectResponse.Code, disconnectResponse.Body.String())
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/local-api/auth/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+localToken)
	meResponse := httptest.NewRecorder()
	handler.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("auth me status = %d; body=%s", meResponse.Code, meResponse.Body.String())
	}
	var me cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(meResponse.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode auth me response: %v", err)
	}
	if me.CloudSession != nil {
		t.Fatalf("cloud_session after disconnect = %#v, want nil", me.CloudSession)
	}
	loaded, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice(reload) error = %v", err)
	}
	if loaded.Id != device.Id || loaded.Name != device.Name || loaded.PublicKey != device.PublicKey {
		t.Fatalf("device identity changed after cloud disconnect: before=%#v after=%#v", device, loaded)
	}
	if loaded.CloudBinding != nil {
		t.Fatalf("cloud binding after disconnect = %#v, want nil", loaded.CloudBinding)
	}
	if len(cloudRequestPaths) != 1 || cloudRequestPaths[0] != "POST /cloud-api/devices/current" {
		t.Fatalf("cloud requests = %#v, want only device report and no unbind/delete", cloudRequestPaths)
	}
	if len(sessionEvents) != 2 || sessionEvents[0] == nil || sessionEvents[1] != nil {
		t.Fatalf("local cloud session events = %#v, want connect summary then nil", sessionEvents)
	}
}

func TestCloudConnectRequiresCloudTokenBeforeDeviceReport(t *testing.T) {
	authService := NewLocalAuthService(sharedauth.NewTokenService(testJWTKey))
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("cloud endpoint should not be called without cloud token")
	}))
	defer cloud.Close()
	handler := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, CloudPublicURL: cloud.URL, LocalDevice: testLocalDevice()})
	localToken := localToken(t, handler)

	request := httptest.NewRequest(http.MethodPost, "/local-api/cloud/connect", strings.NewReader(`{"cloud_token":" "}`))
	request.Header.Set("Authorization", "Bearer "+localToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("cloud connect status = %d, want 400; body=%s", response.Code, response.Body.String())
	}
}

func decodeProtoJSONBody(r *http.Request, message proto.Message) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return codec.UnmarshalProtoJSON(data, message)
}

func localToken(t *testing.T, handler *Handler) string {
	t.Helper()
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/local-api/auth/login", nil))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d; body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var token cloudv1.TokenResp
	if err := codec.UnmarshalProtoJSON(loginResponse.Body.Bytes(), &token); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if token.GetAccessToken() == "" {
		t.Fatalf("login token is empty: token_type=%q", token.GetTokenType())
	}
	return token.AccessToken
}
