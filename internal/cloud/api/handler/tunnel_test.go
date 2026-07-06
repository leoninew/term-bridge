package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	tunnelv1 "termbridge-go/internal/gen/proto/termbridge/tunnel/v1"
	"termbridge-go/internal/shared/dto/protocol/tunnel"
)

func TestAgentTunnelRegistersDevice(t *testing.T) {
	handler := New(Config{Username: "admin", Password: "admin", JWTSecret: testJWTKey, Logger: slog.Default()})
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requestHeader := http.Header{}
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.SetBasicAuth("admin", "admin")
	requestHeader.Set("Authorization", req.Header.Get("Authorization"))
	conn, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/cloud-api/agent/tunnel", &websocket.DialOptions{HTTPHeader: requestHeader})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	hello := &tunnelv1.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &tunnelv1.TunnelFrame_Hello{Hello: &tunnelv1.Hello{DeviceId: "dev-1", DeviceName: "local", ProtocolVersion: tunnel.ProtocolVersion}}}
	helloData, err := tunnel.MarshalFrame(hello)
	if err != nil {
		t.Fatalf("MarshalFrame() error = %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, helloData); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	_, ackData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	ack, err := tunnel.UnmarshalFrame(ackData)
	if err != nil {
		t.Fatalf("UnmarshalFrame() error = %v", err)
	}
	if ack.GetHelloAck() == nil {
		t.Fatalf("ack payload = %T", ack.GetPayload())
	}

	devices := handler.registry.List()
	if len(devices) != 1 || devices[0].Id != "dev-1" || devices[0].Name != "local" || !devices[0].Online {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestDevicesEndpointReturnsRegisteredDevices(t *testing.T) {
	handler := New(testCloudConfig())
	handler.registry.Register("dev-1", "local", time.Now().UTC())
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/cloud-api/auth/login", stringsReader(`{"username":"admin","password":"admin"}`)))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginResponse.Code)
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/cloud-api/devices", nil)
	request.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("devices status = %d", response.Code)
	}
	var devices ListDevicesResp
	if err := json.Unmarshal(response.Body.Bytes(), &devices); err != nil {
		t.Fatalf("Unmarshal() error = %v; body=%s", err, response.Body.String())
	}
	if len(devices.Items) != 1 || devices.Items[0].Id != "dev-1" || !devices.Items[0].Online {
		t.Fatalf("devices = %#v", devices)
	}
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}
