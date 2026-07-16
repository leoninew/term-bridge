package cloudapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	cloudproto "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
)

func TestAuthMeUsesConfiguredApiBaseUrlAndCloudBearer(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/auth/me" {
			t.Fatalf("request method=%s path=%s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("authorization = %q", got)
		}
		body, err := codec.MarshalProtoJSON(&cloudproto.AuthMeResp{Authenticated: true, User: &cloudproto.User{Id: "user-1", Email: "user@example.test"}})
		if err != nil {
			t.Fatalf("encode auth me response: %v", err)
		}
		_, _ = w.Write(body)
	}))
	defer cloud.Close()

	client := New(Config{ApiBaseUrl: cloud.URL + "/api", HttpClient: http.DefaultClient})
	result, err := client.AuthMe(context.Background(), "cloud-token")
	if err != nil {
		t.Fatalf("AuthMe() error = %v", err)
	}
	if !result.GetAuthenticated() || result.GetUser().GetId() != "user-1" || result.GetUser().GetEmail() != "user@example.test" {
		t.Fatalf("auth me result = %#v", result)
	}
}

func TestAuthMeNormalizesCloudUnauthorizedToUnauthenticated(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer cloud.Close()

	client := New(Config{ApiBaseUrl: cloud.URL + "/api", HttpClient: http.DefaultClient})
	result, err := client.AuthMe(context.Background(), "expired-token")
	if err != nil {
		t.Fatalf("AuthMe() error = %v", err)
	}
	if result.GetAuthenticated() || result.GetUser() != nil {
		t.Fatalf("auth me result = %#v", result)
	}
}

func TestAuthMeRejectsInvalidCloudResponse(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"authenticated":`))
	}))
	defer cloud.Close()

	client := New(Config{ApiBaseUrl: cloud.URL + "/api", HttpClient: http.DefaultClient})
	if _, err := client.AuthMe(context.Background(), "cloud-token"); err == nil {
		t.Fatal("AuthMe() error = nil")
	}
}

func TestRegisterCurrentDeviceUsesConfiguredApiBaseUrl(t *testing.T) {
	var deviceReport cloudproto.CurrentDeviceReq
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/devices/current" {
			t.Fatalf("request method=%s path=%s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("authorization = %q", got)
		}
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

	client := New(Config{ApiBaseUrl: cloud.URL + "/api", HttpClient: http.DefaultClient})
	device := agentapp.Device{Id: "device-1", Name: "developer-machine", PublicKey: "public-key"}
	if err := client.RegisterCurrentDevice(context.Background(), "cloud-token", device); err != nil {
		t.Fatalf("RegisterCurrentDevice() error = %v", err)
	}
	if deviceReport.GetId() != device.Id || deviceReport.GetName() != device.Name || deviceReport.GetPublicKey() != device.PublicKey {
		t.Fatalf("device report id=%q name=%q public_key=%q", deviceReport.GetId(), deviceReport.GetName(), deviceReport.GetPublicKey())
	}
}
