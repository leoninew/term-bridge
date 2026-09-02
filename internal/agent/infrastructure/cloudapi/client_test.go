package cloudapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	cloudproto "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/requestid"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const testRequestId = "req_agent_request"

func testContext() context.Context {
	return requestid.With(context.Background(), testRequestId)
}

func TestAuthMeUsesConfiguredApiBaseUrlCloudBearerAndRequestId(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/auth/me" {
			t.Fatalf("request method=%s path=%s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("authorization = %q", got)
		}
		assertRequestId(t, r, testRequestId)
		writeProtoJSON(t, w, &cloudproto.AuthMeResp{Authenticated: true, User: &cloudproto.User{Id: "user-1", Email: "user@example.test"}})
	}))
	defer cloud.Close()

	client := newTestClient(cloud.URL)
	result, err := client.AuthMe(testContext(), "cloud-token")
	if err != nil {
		t.Fatalf("AuthMe() error = %v", err)
	}
	if !result.GetAuthenticated() || result.GetUser().GetId() != "user-1" || result.GetUser().GetEmail() != "user@example.test" {
		t.Fatalf("auth me result = %#v", result)
	}
}

func TestAuthMeKeepsNormalUnauthenticatedResponse(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestId(t, r, testRequestId)
		writeProtoJSON(t, w, &cloudproto.AuthMeResp{Authenticated: false})
	}))
	defer cloud.Close()

	result, err := newTestClient(cloud.URL).AuthMe(testContext(), "expired-token")
	if err != nil {
		t.Fatalf("AuthMe() error = %v", err)
	}
	if result.GetAuthenticated() || result.GetUser() != nil {
		t.Fatalf("auth me result = %#v", result)
	}
}

func TestListDevicesUsesConfiguredApiBaseUrlCloudBearerAndRequestId(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/devices" {
			t.Fatalf("request method=%s path=%s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("authorization = %q", got)
		}
		assertRequestId(t, r, testRequestId)
		writeProtoJSON(t, w, &cloudproto.ListDevicesResp{Items: []*cloudproto.DeviceSummary{{Id: "device-1", Name: "developer-machine", Online: true}}})
	}))
	defer cloud.Close()

	client := newTestClient(cloud.URL)
	result, err := client.ListDevices(testContext(), "cloud-token")
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}
	devices := result.GetItems()
	if len(devices) != 1 || devices[0].GetId() != "device-1" || devices[0].GetName() != "developer-machine" {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestRegisterCurrentDeviceUsesConfiguredApiBaseUrlCloudBearerAndRequestId(t *testing.T) {
	var deviceReport cloudproto.CurrentDeviceReq
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/devices/current" {
			t.Fatalf("request method=%s path=%s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("authorization = %q", got)
		}
		assertRequestId(t, r, testRequestId)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read current device request: %v", err)
		}
		if err := codec.UnmarshalProtoJSON(body, &deviceReport); err != nil {
			t.Fatalf("decode current device request: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer cloud.Close()

	client := newTestClient(cloud.URL)
	device := agentapp.Device{Id: "device-1", Name: "developer-machine", PublicKey: "public-key"}
	if err := client.RegisterCurrentDevice(testContext(), "cloud-token", device); err != nil {
		t.Fatalf("RegisterCurrentDevice() error = %v", err)
	}
	if deviceReport.GetId() != device.Id || deviceReport.GetName() != device.Name || deviceReport.GetPublicKey() != device.PublicKey {
		t.Fatalf("device report id=%q name=%q public_key=%q", deviceReport.GetId(), deviceReport.GetName(), deviceReport.GetPublicKey())
	}
}

func TestExchangeOAuthCodeForwardsRequestId(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/oauth2/token" {
			t.Fatalf("request method=%s path=%s", r.Method, r.URL.Path)
		}
		assertRequestId(t, r, testRequestId)
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token form: %v", err)
		}
		if got := r.Form.Get("code"); got != "oauth-code" {
			t.Fatalf("code = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"exchanged-cloud-token","token_type":"bearer","expires_in":3600}`))
	}))
	defer cloud.Close()

	accessToken, err := newTestClient(cloud.URL).ExchangeOAuthCode(testContext(), testOAuthClient(), "oauth-code")
	if err != nil {
		t.Fatalf("ExchangeOAuthCode() error = %v", err)
	}
	if accessToken != "exchanged-cloud-token" {
		t.Fatalf("access token = %q", accessToken)
	}
}

func TestCloudOperationsPreserveStructuredErrors(t *testing.T) {
	tests := []struct {
		name      string
		operation agentapp.CloudOperation
		status    int
		invoke    func(*Client) error
	}{
		{
			name:      "register current device",
			operation: agentapp.CloudOperationRegisterCurrentDevice,
			status:    http.StatusConflict,
			invoke: func(client *Client) error {
				return client.RegisterCurrentDevice(testContext(), "cloud-token", agentapp.Device{Id: "device-1", Name: "developer-machine", PublicKey: "public-key"})
			},
		},
		{
			name:      "auth me",
			operation: agentapp.CloudOperationAuthMe,
			status:    http.StatusServiceUnavailable,
			invoke: func(client *Client) error {
				_, err := client.AuthMe(testContext(), "cloud-token")
				return err
			},
		},
		{
			name:      "list devices",
			operation: agentapp.CloudOperationListDevices,
			status:    http.StatusUnauthorized,
			invoke: func(client *Client) error {
				_, err := client.ListDevices(testContext(), "cloud-token")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assertRequestId(t, r, testRequestId)
				writeCloudError(t, w, test.status, r.Header.Get(requestIdHeader), "cloud_rejected", "Cloud rejected the request.")
			}))
			defer cloud.Close()

			cloudErr := assertCloudUpstreamError(t, test.invoke(newTestClient(cloud.URL)), test.operation, test.status, "cloud_rejected", "Cloud rejected the request.")
			if cloudErr.Cause == nil {
				t.Fatal("CloudUpstreamError cause = nil")
			}
		})
	}
}

func TestExchangeOAuthCodePreservesStructuredError(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestId(t, r, testRequestId)
		writeCloudError(t, w, http.StatusBadRequest, r.Header.Get(requestIdHeader), "invalid_authorization_code", "Authorization code is invalid.")
	}))
	defer cloud.Close()

	_, err := newTestClient(cloud.URL).ExchangeOAuthCode(testContext(), testOAuthClient(), "bad-code")
	cloudErr := assertCloudUpstreamError(t, err, agentapp.CloudOperationExchangeOAuthCode, http.StatusBadRequest, "invalid_authorization_code", "Authorization code is invalid.")
	if cloudErr.Cause == nil {
		t.Fatal("CloudUpstreamError cause = nil")
	}
}

func TestCloudResponseFailuresRemainTyped(t *testing.T) {
	t.Run("malformed non-success response keeps upstream status", func(t *testing.T) {
		cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertRequestId(t, r, testRequestId)
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer cloud.Close()

		_, err := newTestClient(cloud.URL).ListDevices(testContext(), "cloud-token")
		cloudErr := assertCloudUpstreamError(t, err, agentapp.CloudOperationListDevices, http.StatusTeapot, "", "")
		if cloudErr.CloudError != nil {
			t.Fatalf("CloudError = %#v, want nil", cloudErr.CloudError)
		}
	})

	t.Run("transport failure has no upstream status", func(t *testing.T) {
		client := New(Config{
			ApiBaseUrl: "https://cloud.example.test/api",
			HttpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("cloud unavailable")
			})},
		})

		_, err := client.ListDevices(testContext(), "cloud-token")
		cloudErr := assertCloudUpstreamError(t, err, agentapp.CloudOperationListDevices, 0, "", "")
		if cloudErr.CloudError != nil {
			t.Fatalf("CloudError = %#v, want nil", cloudErr.CloudError)
		}
	})

	t.Run("invalid successful response keeps response status", func(t *testing.T) {
		cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertRequestId(t, r, testRequestId)
			_, _ = w.Write([]byte(`{"authenticated":`))
		}))
		defer cloud.Close()

		_, err := newTestClient(cloud.URL).AuthMe(testContext(), "cloud-token")
		cloudErr := assertCloudUpstreamError(t, err, agentapp.CloudOperationAuthMe, http.StatusOK, "", "")
		if cloudErr.CloudError != nil {
			t.Fatalf("CloudError = %#v, want nil", cloudErr.CloudError)
		}
	})
}

func newTestClient(cloudUrl string) *Client {
	return New(Config{ApiBaseUrl: cloudUrl + "/api", HttpClient: http.DefaultClient})
}

func testOAuthClient() agentapp.OAuthClientConfig {
	return agentapp.OAuthClientConfig{ClientId: "agent-client", ClientSecret: "agent-secret", RedirectUrl: "http://localhost:9030/oauth/callback"}
}

func assertRequestId(t *testing.T, request *http.Request, want string) {
	t.Helper()
	if got := request.Header.Get(requestIdHeader); got != want {
		t.Fatalf("request ID = %q, want %q", got, want)
	}
}

func assertCloudUpstreamError(t *testing.T, err error, operation agentapp.CloudOperation, status int, code string, message string) *agentapp.CloudUpstreamError {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil")
	}
	var cloudErr *agentapp.CloudUpstreamError
	if !errors.As(err, &cloudErr) {
		t.Fatalf("error type = %T, want *CloudUpstreamError; error=%v", err, err)
	}
	if cloudErr.Operation != operation || cloudErr.UpstreamStatus != status || cloudErr.CloudCode() != code {
		t.Fatalf("cloud error operation=%q status=%d code=%q", cloudErr.Operation, cloudErr.UpstreamStatus, cloudErr.CloudCode())
	}
	if code == "" {
		return cloudErr
	}
	if cloudErr.CloudError == nil {
		t.Fatal("CloudError = nil")
	}
	if cloudErr.CloudError.GetError() != message {
		t.Fatalf("cloud error message = %q, want %q", cloudErr.CloudError.GetError(), message)
	}
	if cloudErr.CloudError.GetRequestId() != testRequestId {
		t.Fatalf("cloud error request ID = %q, want %q", cloudErr.CloudError.GetRequestId(), testRequestId)
	}
	if cloudErr.CloudError.GetDetails().GetFields()["reason"].GetStringValue() != "test" {
		t.Fatalf("cloud error details = %#v", cloudErr.CloudError.GetDetails())
	}
	return cloudErr
}

func writeProtoJSON(t *testing.T, writer http.ResponseWriter, message proto.Message) {
	t.Helper()
	body, err := codec.MarshalProtoJSON(message)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(body)
}

func writeCloudError(t *testing.T, writer http.ResponseWriter, status int, requestId string, code string, message string) {
	t.Helper()
	details, err := structpb.NewStruct(map[string]any{"reason": "test"})
	if err != nil {
		t.Fatalf("create error details: %v", err)
	}
	body, err := codec.MarshalProtoJSON(&shared.ErrorResp{Code: code, Error: message, RequestId: requestId, Details: details})
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write(body)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
