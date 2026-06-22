package gatewayapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"termbridge-go/internal/protocol/tunnel"
)

func TestAgentTunnelRegistersDevice(t *testing.T) {
	gateway := New(Config{})
	server := httptest.NewServer(gateway)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requestHeader := http.Header{}
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.SetBasicAuth("admin", "admin")
	requestHeader.Set("Authorization", req.Header.Get("Authorization"))
	conn, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/gateway/agent/tunnel", &websocket.DialOptions{HTTPHeader: requestHeader})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	hello, err := tunnel.NewFrame(tunnel.ControlStreamID, tunnel.FrameHello, tunnel.HelloPayload{DeviceID: "dev-1", DeviceName: "local", ProtocolVersion: tunnel.ProtocolVersion})
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
	if len(devices) != 1 || devices[0].ID != "dev-1" || devices[0].Name != "local" || !devices[0].Online {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestDevicesEndpointReturnsRegisteredDevices(t *testing.T) {
	gateway := New(Config{})
	gateway.registry.Register("dev-1", "local", time.Now().UTC())
	loginResponse := httptest.NewRecorder()
	gateway.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/gateway/login", stringsReader(`{"username":"admin","password":"admin"}`)))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginResponse.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/gateway/devices", nil)
	request.AddCookie(loginResponse.Result().Cookies()[0])
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("devices status = %d", response.Code)
	}
	var devices []DeviceSummary
	if err := json.Unmarshal(response.Body.Bytes(), &devices); err != nil {
		t.Fatalf("Unmarshal() error = %v; body=%s", err, response.Body.String())
	}
	if len(devices) != 1 || devices[0].ID != "dev-1" || !devices[0].Online {
		t.Fatalf("devices = %#v", devices)
	}
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}
