package gatewayapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	agentapp "termbridge-go/internal/application/agent"
	"termbridge-go/internal/protocol/tunnel"
)

func TestAgentTunnelRegistersDevice(t *testing.T) {
	stateDir := t.TempDir()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir, DeviceId: "dev-1", DeviceName: "local"})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	privateKey, err := agentapp.LoadDevicePrivateKey(stateDir, device.Id)
	if err != nil {
		t.Fatalf("LoadDevicePrivateKey() error = %v", err)
	}
	publicKey, err := agentapp.LoadDevicePublicKey(stateDir, device.Id)
	if err != nil {
		t.Fatalf("LoadDevicePublicKey() error = %v", err)
	}
	gateway := New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AgentTunnelAudience: "test-audience", DevicePublicKeys: map[string]ed25519.PublicKey{device.Id: publicKey}})
	server := httptest.NewServer(gateway)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requestHeader, err := agentapp.SignedTunnelHeader(http.MethodGet, "/api/agent/tunnel", "test-audience", device.Id, privateKey, time.Now(), "test-nonce")
	if err != nil {
		t.Fatalf("SignedTunnelHeader() error = %v", err)
	}
	conn, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/agent/tunnel", &websocket.DialOptions{HTTPHeader: requestHeader})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	hello, err := tunnel.NewFrame(tunnel.ControlStreamId, tunnel.FrameHello, tunnel.HelloPayload{DeviceId: "dev-1", DeviceName: "local", ProtocolVersion: tunnel.ProtocolVersion})
	if err != nil {
		t.Fatalf("NewFrame() error = %v", err)
	}
	helloData, err := tunnel.Encode(hello)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, helloData); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	_, ackData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	ack, err := tunnel.Decode(ackData)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if ack.Type != tunnel.FrameHelloAck {
		t.Fatalf("ack type = %s", ack.Type)
	}

	devices := gateway.registry.List()
	if len(devices) != 1 || devices[0].Id != "dev-1" || devices[0].Name != "local" || !devices[0].Online {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestDevicesEndpointReturnsRegisteredDevices(t *testing.T) {
	gateway := New(testGatewayConfig())
	gateway.registry.Register("dev-1", "local", time.Now().UTC())
	loginResponse := httptest.NewRecorder()
	gateway.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/auth/login", stringsReader(`{"username":"admin","password":"admin"}`)))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginResponse.Code)
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	request.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("devices status = %d", response.Code)
	}
	var devices []DeviceSummary
	if err := json.Unmarshal(response.Body.Bytes(), &devices); err != nil {
		t.Fatalf("Unmarshal() error = %v; body=%s", err, response.Body.String())
	}
	if len(devices) != 1 || devices[0].Id != "dev-1" || !devices[0].Online {
		t.Fatalf("devices = %#v", devices)
	}
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}
