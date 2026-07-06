package api

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

	runtimev1 "termbridge-go/internal/gen/proto/termbridge/runtime/v1"
	tunnelv1 "termbridge-go/internal/gen/proto/termbridge/tunnel/v1"
	"termbridge-go/internal/shared/dto/protocol/tunnel"
)

func TestBrowserAPIRelay(t *testing.T) {
	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runFakeAgent(t, ctx, server.URL, func(frame *tunnelv1.TunnelFrame) *tunnelv1.TunnelFrame {
			if frame.GetRequestId() != "req_test_relay" {
				return tunnel.ErrorFrame(frame.GetStreamId(), frame.GetRequestId(), "bad_request", "missing request id")
			}
			switch payload := frame.GetPayload().(type) {
			case *tunnelv1.TunnelFrame_WorkspaceTreeReq:
				return responseFrame(frame, &tunnelv1.TunnelFrame_WorkspaceTreeResp{WorkspaceTreeResp: &runtimev1.WorkspaceTreeResp{Items: []*runtimev1.WorkspaceTreeNode{{Id: "ws-1", Name: "Workspace"}}}})
			case *tunnelv1.TunnelFrame_WorkspaceSessionsReq:
				return responseFrame(frame, &tunnelv1.TunnelFrame_WorkspaceSessionsResp{WorkspaceSessionsResp: &runtimev1.WorkspaceSessionsResp{Items: []*runtimev1.SessionSummary{{Id: "sess-1", WorkspaceId: payload.WorkspaceSessionsReq.GetWorkspaceId(), Name: "Session"}}}})
			case *tunnelv1.TunnelFrame_ReadHistoryReq:
				return responseFrame(frame, &tunnelv1.TunnelFrame_ReadHistoryResp{ReadHistoryResp: &runtimev1.ReadHistoryResp{Text: strings.Repeat("h", 64*1024)}})
			default:
				return tunnel.ErrorFrame(frame.GetStreamId(), frame.GetRequestId(), "bad_request", "unknown payload")
			}
		})
	}()
	waitForRoute(t, handler, "dev-1")
	token := loginToken(t, handler)
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/cloud-api/devices/dev-1/workspaces/tree", "Workspace"},
		{"/cloud-api/devices/dev-1/workspaces/ws-1/sessions", "sess-1"},
		{"/cloud-api/devices/dev-1/workspaces/ws-1/sessions/sess-1/history", strings.Repeat("h", 64*1024)},
	} {
		request := httptest.NewRequest(http.MethodGet, tc.path, nil)
		request.Header.Set(requestIdHeader, "req_test_relay")
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
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

func loginToken(t *testing.T, handler *Handler) string {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/cloud-api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"admin"}`)))
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

func waitForRoute(t *testing.T, handler *Handler, deviceId string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if handler.routeFor(deviceId) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("route %s not registered", deviceId)
}

func runFakeAgent(t *testing.T, ctx context.Context, serverUrl string, respond func(*tunnelv1.TunnelFrame) *tunnelv1.TunnelFrame) {
	t.Helper()
	requestHeader := http.Header{}
	req, _ := http.NewRequest(http.MethodGet, serverUrl, nil)
	req.SetBasicAuth("admin", "admin")
	requestHeader.Set("Authorization", req.Header.Get("Authorization"))
	conn, _, err := websocket.Dial(ctx, "ws"+serverUrl[len("http"):]+"/cloud-api/agent/tunnel", &websocket.DialOptions{HTTPHeader: requestHeader})
	if err != nil {
		t.Errorf("Dial() error = %v", err)
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	hello := &tunnelv1.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &tunnelv1.TunnelFrame_Hello{Hello: &tunnelv1.Hello{DeviceId: "dev-1", DeviceName: "local", ProtocolVersion: tunnel.ProtocolVersion}}}
	helloData, _ := tunnel.MarshalFrame(hello)
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
		frame, err := tunnel.UnmarshalFrame(data)
		if err != nil || !isRuntimeRequestFrame(frame) {
			continue
		}
		responseData, _ := tunnel.MarshalFrame(respond(frame))
		_ = conn.Write(ctx, websocket.MessageText, responseData)
	}
}

func responseFrame(request *tunnelv1.TunnelFrame, payload any) *tunnelv1.TunnelFrame {
	frame := &tunnelv1.TunnelFrame{StreamId: request.GetStreamId(), RequestId: request.GetRequestId()}
	switch value := payload.(type) {
	case *tunnelv1.TunnelFrame_WorkspaceTreeResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_WorkspaceSessionsResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_ReadHistoryResp:
		frame.Payload = value
	}
	return frame
}

func isRuntimeRequestFrame(frame *tunnelv1.TunnelFrame) bool {
	switch frame.GetPayload().(type) {
	case *tunnelv1.TunnelFrame_WorkspaceTreeReq, *tunnelv1.TunnelFrame_WorkspaceSessionsReq, *tunnelv1.TunnelFrame_ReadHistoryReq:
		return true
	default:
		return false
	}
}
