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

	client := New(Config{ApiBaseUrl: cloud.URL, HttpClient: http.DefaultClient})
	device := agentapp.Device{Id: "device-1", Name: "developer-machine", PublicKey: "public-key"}
	if err := client.RegisterCurrentDevice(context.Background(), "cloud-token", device); err != nil {
		t.Fatalf("RegisterCurrentDevice() error = %v", err)
	}
	if deviceReport.GetId() != device.Id || deviceReport.GetName() != device.Name || deviceReport.GetPublicKey() != device.PublicKey {
		t.Fatalf("device report id=%q name=%q public_key=%q", deviceReport.GetId(), deviceReport.GetName(), deviceReport.GetPublicKey())
	}
}
