package gatewayapi

import (
	"bytes"
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

func TestBrowserAPIRelay(t *testing.T) {
	gateway := New(testGatewayConfig())
	server := httptest.NewServer(gateway)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runFakeAgent(t, ctx, server.URL, func(frame tunnel.Frame) tunnel.ResponseResp {
			request, err := tunnel.DecodePayload[tunnel.RequestReq](frame)
			if err != nil {
				return tunnel.ResponseResp{OK: false, Error: err.Error()}
			}
			if request.RequestId != "req_test_relay" {
				return tunnel.ResponseResp{OK: false, Error: "missing request id"}
			}
			switch request.Method {
			case "workspace_tree":
				return rawResponse(`{"items":[{"id":"ws-1","name":"Workspace"}]}`)
			case "workspace_sessions":
				return rawResponse(`{"items":[{"id":"sess-1","name":"Session"}]}`)
			case "history":
				return rawResponse(`"` + strings.Repeat("h", 64*1024) + `"`)
			default:
				return tunnel.ResponseResp{OK: false, Error: "unknown method"}
			}
		})
	}()
	waitForRoute(t, gateway, "dev-1")
	token := loginToken(t, gateway)
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/api/devices/dev-1/workspaces/tree", "Workspace"},
		{"/api/devices/dev-1/workspaces/ws-1/sessions", "sess-1"},
		{"/api/devices/dev-1/workspaces/ws-1/sessions/sess-1/history", strings.Repeat("h", 64*1024)},
	} {
		request := httptest.NewRequest(http.MethodGet, tc.path, nil)
		request.Header.Set(requestIdHeader, "req_test_relay")
		request.Header.Set("Authorization", "Bearer "+token)
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

func rawResponse(raw string) tunnel.ResponseResp {
	return tunnel.ResponseResp{OK: true, Result: []byte(raw)}
}

func loginToken(t *testing.T, gateway *Handler) string {
	t.Helper()
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"admin"}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d; body=%s", response.Code, response.Body.String())
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	return tokenResp.AccessToken
}

func waitForRoute(t *testing.T, gateway *Handler, deviceId string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if gateway.routeFor(deviceId) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("route %s not registered", deviceId)
}

func runFakeAgent(t *testing.T, ctx context.Context, serverUrl string, respond func(tunnel.Frame) tunnel.ResponseResp) {
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
		if err != nil || frame.Type != tunnel.FrameRequest {
			continue
		}
		response, _ := tunnel.NewFrame(frame.StreamId, tunnel.FrameResponse, respond(frame))
		responseData, _ := tunnel.Encode(response)
		_ = conn.Write(ctx, websocket.MessageText, responseData)
	}
}
