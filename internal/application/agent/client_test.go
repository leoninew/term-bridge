package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"termbridge-go/internal/protocol/tunnel"
)

func TestTunnelURL(t *testing.T) {
	if got := tunnelURL("http://127.0.0.1:8080"); got != "ws://127.0.0.1:8080/api/gateway/agent/tunnel" {
		t.Fatalf("tunnelURL(http) = %q", got)
	}
	if got := tunnelURL("https://example.com/base/"); got != "wss://example.com/base/api/gateway/agent/tunnel" {
		t.Fatalf("tunnelURL(https) = %q", got)
	}
}

func TestClientRunSendsHello(t *testing.T) {
	stateDir := t.TempDir()
	helloCh := make(chan tunnel.HelloPayload, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "admin" || password != "admin" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		frame, err := tunnel.Decode(data)
		if err != nil || frame.Type != tunnel.FrameHello {
			return
		}
		payload, err := tunnel.DecodePayload[tunnel.HelloPayload](frame)
		if err != nil {
			return
		}
		helloCh <- payload
		ack, _ := tunnel.NewFrame(tunnel.ControlStreamID, tunnel.FrameHelloAck, tunnel.HelloAckPayload{ProtocolVersion: tunnel.ProtocolVersion})
		ackData, _ := tunnel.Encode(ack)
		_ = conn.Write(r.Context(), websocket.MessageText, ackData)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	client := New(Config{ServerURL: server.URL, DeviceName: "local", StateDir: stateDir})
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Run(ctx)
	}()
	select {
	case hello := <-helloCh:
		if hello.DeviceID == "" || hello.DeviceName != "local" || hello.ProtocolVersion != tunnel.ProtocolVersion {
			t.Fatalf("hello = %#v", hello)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hello")
	}
	cancel()
	select {
	case err := <-errCh:
		if err != context.Canceled && !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "closed network connection") {
			t.Fatalf("Run() error = %v, want context cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to stop")
	}
}
