package gatewayapi

import (
	"context"
	"crypto/ed25519"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	agentapp "termbridge-go/internal/application/agent"
	terminalapp "termbridge-go/internal/application/terminal"
	"termbridge-go/internal/domain/session"
)

func TestGatewayAgentTerminalAttachE2E(t *testing.T) {
	gateway := New(testGatewayConfig())
	server := httptest.NewServer(gateway)
	defer server.Close()

	runtime := newFakeRuntimeAccess()
	stateDir := t.TempDir()
	device := writeGatewayE2EDevice(t, stateDir)
	publicKey, err := agentapp.LoadDevicePublicKey(stateDir, device.Id)
	if err != nil {
		t.Fatalf("LoadDevicePublicKey() error = %v", err)
	}
	gateway.config.AgentTunnelAudience = server.URL
	gateway.config.DevicePublicKeys = map[string]ed25519.PublicKey{device.Id: publicKey}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	client := agentapp.New(agentapp.Config{ConnectUrl: server.URL, Username: "admin", Password: "admin", DeviceId: device.Id, DeviceName: device.Name, StateDir: stateDir, Runtime: runtime, Logger: slog.Default()})
	go func() {
		errCh <- client.Run(ctx)
	}()
	waitForRoute(t, gateway, device.Id)

	token := loginToken(t, gateway)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	browser, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/devices/"+device.Id+"/workspaces/ws-1/sessions/sess-1/ws?cols=144&rows=44", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer browser.Close(websocket.StatusNormalClosure, "")

	stream := runtime.waitAttach(t, "sess-1")
	stream.waitResize(t, 144, 44)
	stream.sendBinary("POMELO_M6_ATTACH_READY\r\n")
	messageType, output, err := browser.Read(ctx)
	if err != nil {
		t.Fatalf("browser Read() error = %v", err)
	}
	if messageType != websocket.MessageBinary || !strings.Contains(string(output), "POMELO_M6_ATTACH_READY") {
		t.Fatalf("terminal output type=%v data=%q, want attach ready marker", messageType, output)
	}

	if err := browser.Write(ctx, websocket.MessageText, []byte(`{"type":"hello"}`)); err != nil {
		t.Fatalf("hello Write() error = %v", err)
	}
	if err := browser.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":132,"rows":35}`)); err != nil {
		t.Fatalf("resize Write() error = %v", err)
	}
	stream.waitResize(t, 132, 35)

	if err := browser.Write(ctx, websocket.MessageBinary, []byte("echo POMELO_M6_ATTACH_INPUT\r\n")); err != nil {
		t.Fatalf("input Write() error = %v", err)
	}
	stream.waitInput(t, "POMELO_M6_ATTACH_INPUT")
	stream.sendBinary("POMELO_M6_ATTACH_INPUT\r\n")
	_, echo, err := browser.Read(ctx)
	if err != nil {
		t.Fatalf("browser echo Read() error = %v", err)
	}
	if !strings.Contains(string(echo), "POMELO_M6_ATTACH_INPUT") {
		t.Fatalf("echo output = %q, want input marker", echo)
	}

	if err := browser.Write(ctx, websocket.MessageText, []byte(`{"type":"detach"}`)); err != nil {
		t.Fatalf("detach Write() error = %v", err)
	}
	stream.waitDetach(t, "gateway_detached")

	cancel()
	select {
	case err := <-errCh:
		if err != nil && !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "closed network connection") {
			t.Fatalf("client Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for agent client to stop")
	}
}

func TestGatewayAgentTerminalAttachResizeErrorReturnsControlError(t *testing.T) {
	gateway := New(testGatewayConfig())
	server := httptest.NewServer(gateway)
	defer server.Close()

	runtime := newFakeRuntimeAccess()
	runtime.newStream = func() *fakeTerminalStream {
		stream := newFakeTerminalStream()
		stream.resizeErrs = []error{errors.New("resize failed")}
		return stream
	}
	stateDir := t.TempDir()
	device := writeGatewayE2EDevice(t, stateDir)
	publicKey, err := agentapp.LoadDevicePublicKey(stateDir, device.Id)
	if err != nil {
		t.Fatalf("LoadDevicePublicKey() error = %v", err)
	}
	gateway.config.AgentTunnelAudience = server.URL
	gateway.config.DevicePublicKeys = map[string]ed25519.PublicKey{device.Id: publicKey}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	client := agentapp.New(agentapp.Config{ConnectUrl: server.URL, Username: "admin", Password: "admin", DeviceId: device.Id, DeviceName: device.Name, StateDir: stateDir, Runtime: runtime, Logger: slog.Default()})
	go func() {
		errCh <- client.Run(ctx)
	}()
	waitForRoute(t, gateway, device.Id)

	token := loginToken(t, gateway)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	browser, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):]+"/api/devices/"+device.Id+"/workspaces/ws-1/sessions/sess-1/ws?cols=144&rows=44", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("browser Dial() error = %v", err)
	}
	defer browser.Close(websocket.StatusNormalClosure, "")

	stream := runtime.waitAttach(t, "sess-1")
	stream.waitResize(t, 144, 44)
	readCtx, cancelRead := context.WithTimeout(ctx, 2*time.Second)
	defer cancelRead()
	messageType, data, err := browser.Read(readCtx)
	if err != nil {
		t.Fatalf("browser Read() error = %v", err)
	}
	if messageType != websocket.MessageText || !strings.Contains(string(data), "terminal_stream_error") || !strings.Contains(string(data), "resize failed") {
		t.Fatalf("terminal error type=%v data=%q, want resize failure control error", messageType, data)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "closed network connection") {
			t.Fatalf("client Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for agent client to stop")
	}
}

func writeGatewayE2EDevice(t *testing.T, stateDir string) agentapp.Device {
	t.Helper()
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: stateDir, DeviceId: "gateway-e2e-device", DeviceName: "gateway-e2e", Now: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	return device
}

type fakeRuntimeAccess struct {
	mu        sync.Mutex
	streams   map[string]*fakeTerminalStream
	attach    chan attachedStream
	newStream func() *fakeTerminalStream
}

type attachedStream struct {
	sessionId string
	stream    *fakeTerminalStream
}

func newFakeRuntimeAccess() *fakeRuntimeAccess {
	return &fakeRuntimeAccess{streams: map[string]*fakeTerminalStream{}, attach: make(chan attachedStream, 1), newStream: newFakeTerminalStream}
}

func (r *fakeRuntimeAccess) ListWorkspaces(context.Context) ([]terminalapp.WorkspaceSummary, error) {
	return []terminalapp.WorkspaceSummary{{Id: "ws-1", Name: "Workspace", Path: "."}}, nil
}

func (r *fakeRuntimeAccess) WorkspaceTree(context.Context) ([]terminalapp.WorkspaceTreeNode, error) {
	return []terminalapp.WorkspaceTreeNode{{Id: "ws-1", Name: "Workspace", Path: ".", Children: []terminalapp.WorkspaceSessionSummary{{Id: "sess-1", Name: "Session", Command: "fake-tui", LifecycleState: session.StateRunning}}}}, nil
}

func (r *fakeRuntimeAccess) UpdateWorkspaceOrder(context.Context, []string) ([]terminalapp.WorkspaceSummary, error) {
	return r.ListWorkspaces(context.Background())
}

func (r *fakeRuntimeAccess) DeleteWorkspace(context.Context, string) error {
	return nil
}

func (r *fakeRuntimeAccess) ListSessionsByWorkspaceId(context.Context, string) ([]terminalapp.WorkspaceSessionSummary, error) {
	return []terminalapp.WorkspaceSessionSummary{{Id: "sess-1", Name: "Session", Command: "fake-tui", LifecycleState: session.StateRunning}}, nil
}

func (r *fakeRuntimeAccess) UpdateSessionOrder(context.Context, string, []string) ([]terminalapp.WorkspaceSessionSummary, error) {
	return r.ListSessionsByWorkspaceId(context.Background(), "ws-1")
}

func (r *fakeRuntimeAccess) CreateSession(context.Context, terminalapp.CreateSessionReq) (terminalapp.CreateSessionResp, error) {
	return terminalapp.CreateSessionResp{SessionId: "sess-1", WorkspaceId: "ws-1", State: string(session.StateRunning)}, nil
}

func (r *fakeRuntimeAccess) RerunSession(context.Context, string, string, terminalapp.RerunSessionReq) (terminalapp.CreateSessionResp, error) {
	return terminalapp.CreateSessionResp{SessionId: "sess-1", WorkspaceId: "ws-1", State: string(session.StateRunning)}, nil
}

func (r *fakeRuntimeAccess) GetSession(context.Context, string, string) (terminalapp.SessionSummary, error) {
	return terminalapp.SessionSummary{Id: "sess-1", WorkspaceId: "ws-1", Name: "Session", Command: "fake-tui", LifecycleState: session.StateRunning}, nil
}

func (r *fakeRuntimeAccess) UpdateSession(context.Context, string, string, terminalapp.UpdateSessionReq) (terminalapp.SessionSummary, error) {
	return r.GetSession(context.Background(), "ws-1", "sess-1")
}

func (r *fakeRuntimeAccess) DeleteSession(context.Context, string, string) error {
	return nil
}

func (r *fakeRuntimeAccess) CloseSession(context.Context, string, string) (terminalapp.SessionSummary, error) {
	return r.GetSession(context.Background(), "ws-1", "sess-1")
}

func (r *fakeRuntimeAccess) ReadHistory(context.Context, string, string) ([]byte, error) {
	return []byte("POMELO_M6_ATTACH_READY\n"), nil
}

func (r *fakeRuntimeAccess) Attach(_ context.Context, workspaceId string, sessionId string) (agentapp.TerminalStream, error) {
	stream := r.newStream()
	r.mu.Lock()
	r.streams[sessionId] = stream
	r.mu.Unlock()
	r.attach <- attachedStream{sessionId: sessionId, stream: stream}
	return stream, nil
}

func (r *fakeRuntimeAccess) waitAttach(t *testing.T, sessionId string) *fakeTerminalStream {
	t.Helper()
	select {
	case attached := <-r.attach:
		if attached.sessionId != sessionId {
			t.Fatalf("attached session = %q, want %q", attached.sessionId, sessionId)
		}
		return attached.stream
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for attach %s", sessionId)
		return nil
	}
}

type fakeTerminalStream struct {
	outbound   chan terminalapp.Outbound
	input      chan []byte
	resize     chan terminalResize
	resizeErrs []error
	detach     chan string
}

type terminalResize struct {
	cols int
	rows int
}

func newFakeTerminalStream() *fakeTerminalStream {
	return &fakeTerminalStream{outbound: make(chan terminalapp.Outbound, 8), input: make(chan []byte, 8), resize: make(chan terminalResize, 8), detach: make(chan string, 1)}
}

func (s *fakeTerminalStream) Outbound() <-chan terminalapp.Outbound {
	return s.outbound
}

func (s *fakeTerminalStream) WriteInput(data []byte) error {
	s.input <- append([]byte(nil), data...)
	return nil
}

func (s *fakeTerminalStream) Resize(cols int, rows int) error {
	s.resize <- terminalResize{cols: cols, rows: rows}
	if len(s.resizeErrs) > 0 {
		err := s.resizeErrs[0]
		s.resizeErrs = s.resizeErrs[1:]
		return err
	}
	return nil
}

func (s *fakeTerminalStream) Detach(reason string) {
	s.detach <- reason
	close(s.outbound)
}

func (s *fakeTerminalStream) sendBinary(value string) {
	s.outbound <- terminalapp.Outbound{Kind: terminalapp.OutboundBinary, Binary: []byte(value)}
}

func (s *fakeTerminalStream) waitInput(t *testing.T, want string) {
	t.Helper()
	select {
	case input := <-s.input:
		if !strings.Contains(string(input), want) {
			t.Fatalf("input = %q, want marker %q", input, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for input %q", want)
	}
}

func (s *fakeTerminalStream) waitResize(t *testing.T, cols int, rows int) {
	t.Helper()
	select {
	case resize := <-s.resize:
		if resize.cols != cols || resize.rows != rows {
			t.Fatalf("resize = %dx%d, want %dx%d", resize.cols, resize.rows, cols, rows)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for resize %dx%d", cols, rows)
	}
}

func (s *fakeTerminalStream) waitDetach(t *testing.T, want string) {
	t.Helper()
	select {
	case reason := <-s.detach:
		if reason != want {
			t.Fatalf("detach reason = %q, want %q", reason, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for detach %q", want)
	}
}

var _ agentapp.RuntimeAccess = (*fakeRuntimeAccess)(nil)
var _ agentapp.TerminalStream = (*fakeTerminalStream)(nil)
