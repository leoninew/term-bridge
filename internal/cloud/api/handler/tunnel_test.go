package api

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

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

func TestAgentTunnelReconnectKeepsDeviceOnline(t *testing.T) {
	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx := t.Context()
	first, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/agent/tunnel", &websocket.DialOptions{HTTPHeader: signedTestTunnelHeader(t)})
	if err != nil {
		t.Fatalf("first Dial() error = %v", err)
	}
	writeHello(t, ctx, first, "dev-1", "local")
	readHelloAck(t, ctx, first)

	second, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/agent/tunnel", &websocket.DialOptions{HTTPHeader: signedTestTunnelHeader(t)})
	if err != nil {
		t.Fatalf("second Dial() error = %v", err)
	}
	defer func() { _ = second.Close(websocket.StatusNormalClosure, "") }()
	writeHello(t, ctx, second, "dev-1", "local")
	readHelloAck(t, ctx, second)

	// Closing the replaced tunnel must not leave the device offline while the
	// replacement route is still active.
	_ = first.Close(websocket.StatusNormalClosure, "replaced")
	deadline := time.Now().Add(2 * time.Second)
	for {
		device, ok := handler.registry.Get("dev-1")
		if !ok {
			t.Fatal("device missing from registry after reconnect")
		}
		online := device.GetOnline()
		route := handler.routeFor("dev-1")
		summary := handler.deviceSummary(Device{Id: "dev-1", Name: "local"})
		if online && route != nil && summary.GetOnline() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("after reconnect: registry_online=%v route=%v summary_online=%v", online, route != nil, summary.GetOnline())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestClearRouteOnlyMarksCurrentOwner(t *testing.T) {
	handler := New(testCloudConfig())
	first := newAgentRoute("dev-1", nil)
	second := newAgentRoute("dev-1", nil)
	handler.registry.Register("dev-1", "local", time.Now().UTC())
	handler.setRoute("dev-1", first)
	handler.setRoute("dev-1", second)

	if cleared := handler.clearRoute("dev-1", first); cleared {
		t.Fatal("clearRoute(old) cleared current route")
	}
	if handler.routeFor("dev-1") != second {
		t.Fatal("current route replaced unexpectedly")
	}
	if cleared := handler.clearRoute("dev-1", second); !cleared {
		t.Fatal("clearRoute(current) did not clear")
	}
	if handler.routeFor("dev-1") != nil {
		t.Fatal("route still present after clear")
	}
}

func TestAgentTunnelIdleTimeoutMarksOffline(t *testing.T) {
	prevInterval := tunnel.HeartbeatInterval
	prevTimeout := tunnel.HeartbeatIdleTimeout
	tunnel.HeartbeatInterval = 40 * time.Millisecond
	tunnel.HeartbeatIdleTimeout = 120 * time.Millisecond
	t.Cleanup(func() {
		tunnel.HeartbeatInterval = prevInterval
		tunnel.HeartbeatIdleTimeout = prevTimeout
	})

	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx := t.Context()
	conn, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/agent/tunnel", &websocket.DialOptions{HTTPHeader: signedTestTunnelHeader(t)})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	writeHello(t, ctx, conn, "dev-1", "local")
	readHelloAck(t, ctx, conn)

	// Stay silent so the cloud-side idle read timeout closes the tunnel.
	deadline := time.Now().Add(2 * time.Second)
	for {
		device, ok := handler.registry.Get("dev-1")
		if ok && !device.GetOnline() && handler.routeFor("dev-1") == nil {
			return
		}
		if time.Now().After(deadline) {
			online := false
			if ok {
				online = device.GetOnline()
			}
			t.Fatalf("device still online after idle timeout: registry_ok=%v online=%v route=%v", ok, online, handler.routeFor("dev-1") != nil)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestAgentTunnelPingKeepsDeviceOnline(t *testing.T) {
	prevInterval := tunnel.HeartbeatInterval
	prevTimeout := tunnel.HeartbeatIdleTimeout
	tunnel.HeartbeatInterval = 40 * time.Millisecond
	tunnel.HeartbeatIdleTimeout = 200 * time.Millisecond
	t.Cleanup(func() {
		tunnel.HeartbeatInterval = prevInterval
		tunnel.HeartbeatIdleTimeout = prevTimeout
	})

	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx := t.Context()
	conn, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/agent/tunnel", &websocket.DialOptions{HTTPHeader: signedTestTunnelHeader(t)})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	writeHello(t, ctx, conn, "dev-1", "local")
	readHelloAck(t, ctx, conn)

	// Act as agent: send pings longer than one idle window.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		ping := tunnel.PingFrame(fmt.Sprintf("%d", time.Now().UnixNano()))
		data, err := tunnel.MarshalFrame(ping)
		if err != nil {
			t.Fatalf("MarshalFrame() error = %v", err)
		}
		if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
			t.Fatalf("Write(ping) error = %v", err)
		}
		_, pongData, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("Read(pong) error = %v", err)
		}
		pong, err := tunnel.UnmarshalFrame(pongData)
		if err != nil {
			t.Fatalf("UnmarshalFrame(pong) error = %v", err)
		}
		if pong.GetPong() == nil {
			t.Fatalf("expected pong, got %T", pong.GetPayload())
		}
		time.Sleep(40 * time.Millisecond)
	}

	device, ok := handler.registry.Get("dev-1")
	if !ok || !device.GetOnline() || handler.routeFor("dev-1") == nil {
		t.Fatalf("device offline while pinging: ok=%v online=%v route=%v", ok, device.GetOnline(), handler.routeFor("dev-1") != nil)
	}
}

func writeHello(t *testing.T, ctx context.Context, conn *websocket.Conn, deviceId string, deviceName string) {
	t.Helper()
	hello := &shared.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &shared.TunnelFrame_Hello{Hello: &shared.Hello{DeviceId: deviceId, DeviceName: deviceName, ProtocolVersion: tunnel.ProtocolVersion}}}
	helloData, err := tunnel.MarshalFrame(hello)
	if err != nil {
		t.Fatalf("MarshalFrame() error = %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, helloData); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
}

func readHelloAck(t *testing.T, ctx context.Context, conn *websocket.Conn) {
	t.Helper()
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
}
