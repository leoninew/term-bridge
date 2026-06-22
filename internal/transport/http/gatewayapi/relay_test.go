package gatewayapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"termbridge-go/internal/protocol/tunnel"
)

func TestBrowserAPIRelay(t *testing.T) {
	gateway := New(Config{})
	server := httptest.NewServer(gateway)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runFakeAgent(t, ctx, server.URL, func(frame tunnel.Frame) tunnel.ResponsePayload {
			request, err := tunnel.DecodePayload[tunnel.RequestPayload](frame)
			if err != nil {
				return tunnel.ResponsePayload{OK: false, Error: err.Error()}
			}
			switch request.Method {
			case "workspace_tree":
				return rawResponse(`[{"id":"ws-1","name":"Workspace"}]`)
			case "sessions":
				return rawResponse(`[{"id":"sess-1","name":"Session"}]`)
			case "history":
				return rawResponse(`"history text"`)
			default:
				return tunnel.ResponsePayload{OK: false, Error: "unknown method"}
			}
		})
	}()
	waitForRoute(t, gateway, "dev-1")
	cookie := loginCookie(t, gateway)
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/api/gateway/devices/dev-1/workspaces/tree", "Workspace"},
		{"/api/gateway/devices/dev-1/sessions", "sess-1"},
		{"/api/gateway/devices/dev-1/sessions/sess-1/history", "history text"},
	} {
		request := httptest.NewRequest(http.MethodGet, tc.path, nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		gateway.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d; body=%s", tc.path, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), tc.want) {
			t.Fatalf("%s body = %s, want %q", tc.path, response.Body.String(), tc.want)
		}
	}
	cancel()
	<-agentDone
}

func rawResponse(raw string) tunnel.ResponsePayload {
	return tunnel.ResponsePayload{OK: true, Result: []byte(raw)}
}

func loginCookie(t *testing.T, gateway *Handler) *http.Cookie {
	t.Helper()
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/gateway/login", bytes.NewBufferString(`{"username":"admin","password":"admin"}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d; body=%s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %#v", cookies)
	}
	return cookies[0]
}

func waitForRoute(t *testing.T, gateway *Handler, deviceID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if gateway.routeFor(deviceID) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("route %s not registered", deviceID)
}

func runFakeAgent(t *testing.T, ctx context.Context, serverURL string, respond func(tunnel.Frame) tunnel.ResponsePayload) {
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
		if err != nil || frame.Type != tunnel.FrameRequest {
			continue
		}
		response, _ := tunnel.NewFrame(frame.StreamID, tunnel.FrameResponse, respond(frame))
		responseData, _ := tunnel.Encode(response)
		_ = conn.Write(ctx, websocket.MessageText, responseData)
	}
}
