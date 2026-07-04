package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	tunnelv1 "termbridge-go/internal/shared/dto/proto/termbridge/tunnel/v1"
	"termbridge-go/internal/shared/dto/protocol/tunnel"
)

func TestTerminalRelayOutputInputAndSingleWriter(t *testing.T) {
	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inputCh := make(chan []byte, 1)
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runTerminalAgent(t, ctx, server.URL, inputCh)
	}()
	waitForRoute(t, handler, "dev-1")
	token := loginToken(t, handler)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	browser, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/cloud-api/devices/dev-1/workspaces/ws-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer func() { _ = browser.Close(websocket.StatusNormalClosure, "") }()
	_, output, err := browser.Read(ctx)
	if err != nil {
		t.Fatalf("browser Read() error = %v", err)
	}
	wantOutput := []byte{'h', 'e', 'l', 'l', 'o', 0xff, 0xfe, 0x1b, '[', '2', 'J'}
	if !bytes.Equal(output, wantOutput) {
		t.Fatalf("terminal output = %q, want %q", output, wantOutput)
	}
	_, response, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/cloud-api/devices/dev-1/workspaces/ws-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
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
		if err != nil {
			continue
		}
		switch payload := frame.GetPayload().(type) {
		case *tunnelv1.TunnelFrame_TerminalAttach:
			if frame.GetRequestId() == "" || payload.TerminalAttach.GetWorkspaceId() == "" {
				return
			}
			output := &tunnelv1.TunnelFrame{StreamId: frame.GetStreamId(), Payload: &tunnelv1.TunnelFrame_TerminalOutput{TerminalOutput: &tunnelv1.TerminalOutput{Data: []byte{'h', 'e', 'l', 'l', 'o', 0xff, 0xfe, 0x1b, '[', '2', 'J'}}}}
			outputData, _ := tunnel.MarshalFrame(output)
			_ = conn.Write(ctx, websocket.MessageText, outputData)
		case *tunnelv1.TunnelFrame_TerminalInput:
			inputCh <- payload.TerminalInput.GetData()
		case *tunnelv1.TunnelFrame_Close:
			return
		}
	}
}

func TestAgentDisconnectSendsStructuredTerminalError(t *testing.T) {
	handler := New(testCloudConfig())
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inputCh := make(chan []byte, 1)
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		runTerminalAgent(t, ctx, server.URL, inputCh)
	}()
	waitForRoute(t, handler, "dev-1")
	token := loginToken(t, handler)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	browser, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/cloud-api/devices/dev-1/workspaces/ws-1/sessions/sess-1/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer func() { _ = browser.Close(websocket.StatusNormalClosure, "") }()
	if _, _, err := browser.Read(ctx); err != nil {
		t.Fatalf("initial output Read() error = %v", err)
	}
	cancel()
	_, data, err := browser.Read(context.Background())
	if err != nil {
		t.Fatalf("terminal error Read() error = %v", err)
	}
	var message struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("terminal error is not JSON: %v; data=%q", err, data)
	}
	if message.Type != "error" || message.Code != "device_disconnected" || message.Message == "" {
		t.Fatalf("terminal error = %#v", message)
	}
	<-agentDone
}

func TestRouteUnavailable(t *testing.T) {
	handler := New(testCloudConfig())
	token := loginToken(t, handler)
	request := httptest.NewRequest(http.MethodGet, "/cloud-api/devices/missing/workspaces/ws-1/sessions", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := assertAPIError(t, response, http.StatusServiceUnavailable, errorCodeDeviceOffline)
	if body.Error != errorMessageDeviceOffline {
		t.Fatalf("error = %q, want %q", body.Error, errorMessageDeviceOffline)
	}
}
