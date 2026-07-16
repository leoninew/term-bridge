package api

import (
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"

	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

func TestAgentTunnelRegistersDevice(t *testing.T) {
	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx := t.Context()
	conn, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/agent/tunnel", &websocket.DialOptions{HTTPHeader: signedTestTunnelHeader(t)})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	hello := &shared.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &shared.TunnelFrame_Hello{Hello: &shared.Hello{DeviceId: "dev-1", DeviceName: "local", ProtocolVersion: tunnel.ProtocolVersion}}}
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
