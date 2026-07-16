package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	cloudapi "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/cloudapi"
	cloudv1 "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/prototime"

	"google.golang.org/protobuf/proto"
)

func testLocalDevice() agentapp.Device {
	return agentapp.Device{Id: "dev-1", Name: "local-device", PublicKey: "public-key"}
}

func requestWithCloudToken(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestAgentStatusReportsRuntimeState(t *testing.T) {
	device := testLocalDevice()
	handler := New(Config{Logger: slog.Default(), LocalDevice: device})
	connectedAt := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	handler.setLocalCloudSession(&cloudv1.CloudSessionSummary{PublicUrl: "https://cloud.example.test", DeviceId: device.Id, DeviceName: device.Name, ConnectedAt: prototime.FromTime(connectedAt)})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/agent/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("agent status = %d; body=%s", response.Code, response.Body.String())
	}
	var body cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode agent status: %v", err)
	}
	if body.GetUser() != nil {
		t.Fatalf("agent status user = %#v, want nil", body.GetUser())
	}
	if body.GetDevice().GetId() != device.Id || body.GetDevice().GetName() != device.Name {
		t.Fatalf("device = %#v", body.GetDevice())
	}
	if body.CloudSession == nil || body.CloudSession.GetPublicUrl() != "https://cloud.example.test" || !prototime.ToTime(body.CloudSession.GetConnectedAt()).Equal(connectedAt) {
		t.Fatalf("cloud_session = %#v", body.CloudSession)
	}
}

func TestCloudAuthMeForwardsCloudTokenFromAuthorizationHeader(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/auth/me" {
			t.Fatalf("cloud request method=%s path=%s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("cloud authorization = %q", got)
		}
		body, err := codec.MarshalProtoJSON(&cloudv1.AuthMeResp{Authenticated: true, User: &cloudv1.User{Id: "cloud-user", Email: "cloud@example.test"}})
		if err != nil {
			t.Fatalf("encode cloud auth me response: %v", err)
		}
		_, _ = w.Write(body)
	}))
	defer cloud.Close()
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api")})
	request := requestWithCloudToken(http.MethodGet, "/api/cloud/auth/me", "cloud-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("cloud auth me status = %d; body=%s", response.Code, response.Body.String())
	}
	var body cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cloud auth me response: %v", err)
	}
	if !body.GetAuthenticated() || body.GetUser().GetId() != "cloud-user" || body.GetUser().GetEmail() != "cloud@example.test" {
		t.Fatalf("cloud auth me authenticated=%v user_id=%q email=%q", body.GetAuthenticated(), body.GetUser().GetId(), body.GetUser().GetEmail())
	}
}

func TestCloudAuthMeRequiresCloudToken(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("cloud endpoint should not be called without Cloud token")
	}))
	defer cloud.Close()
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api")})
	request := httptest.NewRequest(http.MethodGet, "/api/cloud/auth/me", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("cloud auth me status = %d, want 401; body=%s", response.Code, response.Body.String())
	}
}

func TestCloudAuthMeRejectsPost(t *testing.T) {
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService("https://cloud.example.test/api")})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/cloud/auth/me", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("cloud auth me status = %d, want 405; body=%s", response.Code, response.Body.String())
	}
}

func TestCloudAuthMeSoftFailsWhenCloudIsUnavailable(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"cloud failure"}`))
	}))
	defer cloud.Close()
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api")})
	request := requestWithCloudToken(http.MethodGet, "/api/cloud/auth/me", "cloud-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("cloud auth me status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var body cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cloud auth me: %v", err)
	}
	if body.GetAuthenticated() {
		t.Fatalf("authenticated = true, want false when cloud is unavailable")
	}
	if body.GetUser() != nil {
		t.Fatalf("user = %#v, want nil when cloud is unavailable", body.GetUser())
	}
}

func TestCloudConnectReportsCurrentDeviceWithCloudToken(t *testing.T) {
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	var deviceReport cloudv1.CurrentDeviceReq
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/devices/current" {
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
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api"), LocalDevice: device, LocalDeviceStateDir: stateDir})
	request := requestWithCloudToken(http.MethodPost, "/api/cloud/connect", "cloud-token")
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
	if body.CloudSession.GetPublicUrl() != cloud.URL+"/api" || body.CloudSession.GetDeviceId() != device.Id || body.CloudSession.GetDeviceName() != device.Name {
		t.Fatalf("cloud session = %#v", body.CloudSession)
	}
}

func TestCloudConnectDoesNotPersistLocalCloudSession(t *testing.T) {
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/devices/current" {
			t.Fatalf("unexpected cloud request path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer cloud.Close()
	first := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api"), LocalDevice: device, LocalDeviceStateDir: stateDir})
	connectResponse := httptest.NewRecorder()
	first.ServeHTTP(connectResponse, requestWithCloudToken(http.MethodPost, "/api/cloud/connect", "cloud-token"))
	if connectResponse.Code != http.StatusOK {
		t.Fatalf("cloud connect status = %d; body=%s", connectResponse.Code, connectResponse.Body.String())
	}

	second := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api"), LocalDevice: agentapp.Device{Id: device.Id, Name: device.Name, PublicKey: device.PublicKey}, LocalDeviceStateDir: stateDir})
	statusResponse := httptest.NewRecorder()
	second.ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/api/agent/status", nil))
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("agent status = %d; body=%s", statusResponse.Code, statusResponse.Body.String())
	}
	var body cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(statusResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode agent status: %v", err)
	}
	if body.CloudSession != nil {
		t.Fatalf("cloud_session = %#v, want nil after process restart", body.CloudSession)
	}
}

func TestCloudDisconnectClearsRuntimeLocalCloudSession(t *testing.T) {
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer cloud.Close()
	var observed *cloudv1.CloudSessionSummary
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api"), LocalDevice: device, LocalDeviceStateDir: stateDir, OnLocalCloudSession: func(summary *cloudv1.CloudSessionSummary) {
		if summary == nil {
			observed = nil
			return
		}
		observed = proto.Clone(summary).(*cloudv1.CloudSessionSummary)
	}})
	connectResponse := httptest.NewRecorder()
	handler.ServeHTTP(connectResponse, requestWithCloudToken(http.MethodPost, "/api/cloud/connect", "cloud-token"))
	if connectResponse.Code != http.StatusOK {
		t.Fatalf("cloud connect status = %d; body=%s", connectResponse.Code, connectResponse.Body.String())
	}
	if observed == nil || observed.GetDeviceId() != device.Id {
		t.Fatalf("observed session after connect = %#v", observed)
	}

	disconnectResponse := httptest.NewRecorder()
	handler.ServeHTTP(disconnectResponse, httptest.NewRequest(http.MethodPost, "/api/cloud/disconnect", nil))
	if disconnectResponse.Code != http.StatusOK {
		t.Fatalf("cloud disconnect status = %d; body=%s", disconnectResponse.Code, disconnectResponse.Body.String())
	}
	if observed != nil {
		t.Fatalf("observed session after disconnect = %#v, want nil", observed)
	}

	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/api/agent/status", nil))
	var me cloudv1.AuthMeResp
	if err := codec.UnmarshalProtoJSON(statusResponse.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode agent status: %v", err)
	}
	if me.CloudSession != nil {
		t.Fatalf("cloud_session after disconnect = %#v, want nil", me.CloudSession)
	}
}

func TestCloudConnectRequiresCloudTokenBeforeDeviceReport(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("cloud endpoint should not be called without cloud token")
	}))
	defer cloud.Close()
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api"), LocalDevice: testLocalDevice()})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/cloud/connect", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("cloud connect status = %d, want 401; body=%s", response.Code, response.Body.String())
	}
}

func TestExchangeOAuthCodeReturnsAccessToken(t *testing.T) {
	const expectedToken = "exchanged-cloud-token"
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/oauth2/token" {
			t.Fatalf("unexpected cloud token path = %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token request form: %v", err)
		}
		if r.Form.Get("code") != "oauth-code" {
			t.Fatalf("code = %q", r.Form.Get("code"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"` + expectedToken + `","token_type":"bearer","expires_in":3600}`))
	}))
	defer cloud.Close()
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api")})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/cloud/oauth/exchange", strings.NewReader(`{"code":"oauth-code"}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("exchange status = %d; body=%s", response.Code, response.Body.String())
	}
	var body cloudv1.CloudOAuthExchangeResp
	if err := codec.UnmarshalProtoJSON(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode exchange response: %v", err)
	}
	if body.GetAccessToken() != expectedToken || body.GetTokenType() != "bearer" {
		t.Fatalf("exchange response access_token=%q token_type=%q", body.GetAccessToken(), body.GetTokenType())
	}
}

func TestExchangeOAuthCodeRequiresCode(t *testing.T) {
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService("https://cloud.example.test/api")})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/cloud/oauth/exchange", strings.NewReader(`{"code":" "}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("exchange status = %d, want 400; body=%s", response.Code, response.Body.String())
	}
}

func TestExchangeOAuthCodeReturnsBadGatewayOnCloudError(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer cloud.Close()
	handler := New(Config{Logger: slog.Default(), CloudService: testCloudService(cloud.URL + "/api")})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/cloud/oauth/exchange", strings.NewReader(`{"code":"bad-code"}`)))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("exchange status = %d, want 502; body=%s", response.Code, response.Body.String())
	}
}

func testCloudService(apiBaseURL string) *agentapp.CloudService {
	return agentapp.NewCloudService(cloudapi.New(cloudapi.Config{ApiBaseUrl: apiBaseURL, HttpClient: http.DefaultClient}), agentapp.CloudServiceConfig{PublicUrl: apiBaseURL})
}

func decodeProtoJSONBody(r *http.Request, message proto.Message) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return codec.UnmarshalProtoJSON(data, message)
}
