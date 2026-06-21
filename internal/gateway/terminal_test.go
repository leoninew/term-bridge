package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"termbridge-go/internal/tunnel"
)

func TestTerminalRelayOutputInputAndSingleWriter(t *testing.T) {
	gateway := New(Config{})
	server := httptest.NewServer(gateway.server.Handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inputCh := make(chan string, 1)
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runTerminalAgent(t, ctx, server.URL, inputCh)
	}()
	waitForRoute(t, gateway, "dev-1")
	cookie := loginCookie(t, gateway)
	header := http.Header{}
	header.Set("Cookie", cookie.Name+"="+cookie.Value)
	browser, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/gateway/devices/dev-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer browser.Close(websocket.StatusNormalClosure, "")
	_, output, err := browser.Read(ctx)
	if err != nil {
		t.Fatalf("browser Read() error = %v", err)
	}
	if string(output) != "hello" {
		t.Fatalf("terminal output = %q, want hello", output)
	}
	_, response, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/gateway/devices/dev-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err == nil {
		t.Fatal("second writer Dial() error = nil, want conflict")
	}
	if response == nil || response.StatusCode != http.StatusConflict {
		t.Fatalf("second writer status = %#v, want 409", response)
	}
	if err := browser.Write(ctx, websocket.MessageBinary, []byte("input")); err != nil {
		t.Fatalf("browser Write() error = %v", err)
	}
	select {
	case input := <-inputCh:
		if input != "input" {
			t.Fatalf("input = %q, want input", input)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal input")
	}
	cancel()
	<-agentDone
}

func runTerminalAgent(t *testing.T, ctx context.Context, serverURL string, inputCh chan<- string) {
	t.Helper()
	requestHeader := http.Header{}
	req, _ := http.NewRequest(http.MethodGet, serverURL, nil)
	req.SetBasicAuth("admin", "admin")
	requestHeader.Set("Authorization", req.Header.Get("Authorization"))
	conn, _, err := websocket.Dial(ctx, "ws"+serverURL[len("http"):]+"/api/gateway/agent/tunnel", &websocket.DialOptions{HTTPHeader: requestHeader})
	if err != nil {
		t.Errorf("Dial() error = %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	hello, _ := tunnel.NewFrame(tunnel.ControlStreamID, tunnel.FrameHello, tunnel.HelloPayload{DeviceID: "dev-1", DeviceName: "local", ProtocolVersion: tunnel.ProtocolVersion})
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
			output, _ := tunnel.NewFrame(frame.StreamID, tunnel.FrameTerminalOutput, tunnel.TerminalDataPayload{Data: "hello"})
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
	gateway := New(Config{})
	cookie := loginCookie(t, gateway)
	request := httptest.NewRequest(http.MethodGet, "/api/gateway/devices/missing/sessions", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	gateway.server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "route unavailable") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
