package gatewayapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"termbridge-go/internal/protocol/tunnel"
)

func TestTerminalRelayOutputInputAndSingleWriter(t *testing.T) {
	gateway := New(testGatewayConfig())
	server := httptest.NewServer(gateway)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inputCh := make(chan []byte, 1)
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runTerminalAgent(t, ctx, server.URL, inputCh)
	}()
	waitForRoute(t, gateway, "dev-1")
	token := loginToken(t, gateway)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	browser, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/devices/dev-1/workspaces/ws-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer browser.Close(websocket.StatusNormalClosure, "")
	_, output, err := browser.Read(ctx)
	if err != nil {
		t.Fatalf("browser Read() error = %v", err)
	}
	wantOutput := []byte{'h', 'e', 'l', 'l', 'o', 0xff, 0xfe, 0x1b, '[', '2', 'J'}
	if !bytes.Equal(output, wantOutput) {
		t.Fatalf("terminal output = %q, want %q", output, wantOutput)
	}
	_, response, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/devices/dev-1/workspaces/ws-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err == nil {
		t.Fatal("second writer Dial() error = nil, want conflict")
	}
	if response == nil || response.StatusCode != http.StatusConflict {
		t.Fatalf("second writer status = %#v, want 409", response)
	}
	wantInput := []byte{'i', 'n', 'p', 'u', 't', 0xff, 0x00, 0x1b, '[', 'A'}
	if err := browser.Write(ctx, websocket.MessageBinary, wantInput); err != nil {
		t.Fatalf("browser Write() error = %v", err)
	}
	select {
	case input := <-inputCh:
		if !bytes.Equal(input, wantInput) {
			t.Fatalf("input = %q, want %q", input, wantInput)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal input")
	}
	cancel()
	<-agentDone
}

func runTerminalAgent(t *testing.T, ctx context.Context, serverUrl string, inputCh chan<- []byte) {
	t.Helper()
	requestHeader := http.Header{}
	req, _ := http.NewRequest(http.MethodGet, serverUrl, nil)
	req.SetBasicAuth("admin", "admin")
	requestHeader.Set("Authorization", req.Header.Get("Authorization"))
	conn, _, err := websocket.Dial(ctx, "ws"+serverUrl[len("http"):]+"/api/agent/tunnel", &websocket.DialOptions{HTTPHeader: requestHeader})
	if err != nil {
		t.Errorf("Dial() error = %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	hello, _ := tunnel.NewFrame(tunnel.ControlStreamId, tunnel.FrameHello, tunnel.HelloPayload{DeviceId: "dev-1", DeviceName: "local", ProtocolVersion: tunnel.ProtocolVersion})
	helloData, _ := tunnel.Encode(hello)
	if err := conn.Write(ctx, websocket.MessageText, helloData); err != nil {
		t.Errorf("hello Write() error = %v", err)
		return
	}
	if _, _, err := conn.Read(ctx); err != nil {
		t.Errorf("ack Read() error = %v", err)
		return
	}
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		frame, err := tunnel.Decode(data)
		if err != nil {
			continue
		}
		switch frame.Type {
		case tunnel.FrameTerminalAttach:
			attach, err := tunnel.DecodePayload[tunnel.TerminalAttachReq](frame)
			if err != nil || attach.RequestId == "" {
				return
			}
			output, _ := tunnel.NewFrame(frame.StreamId, tunnel.FrameTerminalOutput, tunnel.TerminalDataPayload{Data: []byte{'h', 'e', 'l', 'l', 'o', 0xff, 0xfe, 0x1b, '[', '2', 'J'}})
			outputData, _ := tunnel.Encode(output)
			_ = conn.Write(ctx, websocket.MessageText, outputData)
		case tunnel.FrameTerminalInput:
			payload, err := tunnel.DecodePayload[tunnel.TerminalDataPayload](frame)
			if err == nil {
				inputCh <- payload.Data
			}
		case tunnel.FrameClose:
			return
		}
	}
}

func TestRouteUnavailable(t *testing.T) {
	gateway := New(testGatewayConfig())
	token := loginToken(t, gateway)
	request := httptest.NewRequest(http.MethodGet, "/api/devices/missing/workspaces/ws-1/sessions", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	body := assertAPIError(t, response, http.StatusServiceUnavailable, errorCodeDeviceOffline)
	if body.Error != errorMessageDeviceOffline {
		t.Fatalf("error = %q, want %q", body.Error, errorMessageDeviceOffline)
	}
}
