package application

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/types/known/timestamppb"

	terminalapp "termbridge-go/internal/agent/application/task/terminal"
	runtimev1 "termbridge-go/internal/shared/dto/proto/termbridge/runtime/v1"
	tunnelv1 "termbridge-go/internal/shared/dto/proto/termbridge/tunnel/v1"
	terminalproto "termbridge-go/internal/shared/dto/protocol/terminal"
	"termbridge-go/internal/shared/dto/protocol/tunnel"
)

type Config struct {
	ConnectUrl string
	StateDir   string
	Runtime    RuntimeAccess
	Logger     *slog.Logger
}

type Client struct {
	config Config
	device Device
	mu     sync.Mutex
	terms  map[string]chan *tunnelv1.TunnelFrame
}

func (c *Client) SetDevice(device Device) {
	c.device = device
}

func New(config Config) *Client {
	return &Client{config: config, terms: map[string]chan *tunnelv1.TunnelFrame{}}
}

func (c *Client) logInfo(message string, attrs ...any) {
	if c.config.Logger != nil {
		c.config.Logger.Info(message, attrs...)
	}
}

func (c *Client) logWarn(message string, attrs ...any) {
	if c.config.Logger != nil {
		c.config.Logger.Warn(message, attrs...)
	}
}

func (c *Client) Run(ctx context.Context) error {
	device := c.device
	if strings.TrimSpace(device.Id) == "" {
		loaded, err := LoadOrCreateDevice(DeviceOptions{StateDir: c.config.StateDir})
		if err != nil {
			return err
		}
		device = loaded
	}
	c.device = device
	header, err := c.tunnelHeader(device)
	if err != nil {
		return err
	}
	conn, _, err := websocket.Dial(ctx, tunnelUrl(c.config.ConnectUrl), &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return err
	}
	conn.SetReadLimit(tunnel.MaxFrameBytes)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	go func() {
		<-ctx.Done()
		_ = conn.CloseNow()
	}()
	hello := &tunnelv1.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &tunnelv1.TunnelFrame_Hello{Hello: &tunnelv1.Hello{DeviceId: device.Id, DeviceName: device.Name, ProtocolVersion: tunnel.ProtocolVersion}}}
	helloData, err := tunnel.MarshalFrame(hello)
	if err != nil {
		return err
	}
	if err := conn.Write(ctx, websocket.MessageText, helloData); err != nil {
		return err
	}
	_, ackData, err := conn.Read(ctx)
	if err != nil {
		return err
	}
	ack, err := tunnel.UnmarshalFrame(ackData)
	if err != nil {
		return err
	}
	if ack.GetHelloAck() == nil {
		return websocket.CloseError{Code: websocket.StatusPolicyViolation, Reason: "expected hello_ack"}
	}
	return c.readLoop(ctx, conn)
}

func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn) error {
	var writeMu sync.Mutex
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		frame, err := tunnel.UnmarshalFrame(data)
		if err != nil {
			continue
		}
		if c.dispatchTerminal(frame) {
			continue
		}
		switch frame.GetPayload().(type) {
		case *tunnelv1.TunnelFrame_TerminalAttach:
			go c.handleTerminal(ctx, conn, &writeMu, frame)
		case *tunnelv1.TunnelFrame_Ping:
			pong := &tunnelv1.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &tunnelv1.TunnelFrame_Pong{Pong: &tunnelv1.Pong{Nonce: frame.GetPing().GetNonce()}}}
			_ = writeTunnelFrame(ctx, conn, &writeMu, pong)
		default:
			if isRuntimeRequest(frame) {
				go c.handleRequest(ctx, conn, &writeMu, frame)
			}
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame *tunnelv1.TunnelFrame) {
	attrs := []any{"stream_id", frame.GetStreamId(), "request_id", frame.GetRequestId()}
	if c.config.Runtime == nil {
		err := fmt.Errorf("runtime access unavailable")
		c.logWarn("agent tunnel request failed", append(attrs, "error", err)...)
		_ = writeTunnelFrame(ctx, conn, writeMu, tunnel.ErrorFrame(frame.GetStreamId(), frame.GetRequestId(), "runtime_unavailable", err.Error()))
		return
	}
	response, err := c.handleRuntimeRequest(ctx, frame)
	if err != nil {
		c.logWarn("agent tunnel request failed", append(attrs, "error", err)...)
		_ = writeTunnelFrame(ctx, conn, writeMu, tunnel.ErrorFrame(frame.GetStreamId(), frame.GetRequestId(), "runtime_error", err.Error()))
		return
	}
	c.logInfo("agent tunnel request handled", attrs...)
	_ = writeTunnelFrame(ctx, conn, writeMu, response)
}

func (c *Client) handleRuntimeRequest(ctx context.Context, frame *tunnelv1.TunnelFrame) (*tunnelv1.TunnelFrame, error) {
	return HandleRuntimeRequest(ctx, c.config.Runtime, frame)
}

func HandleRuntimeRequest(ctx context.Context, runtime RuntimeAccess, frame *tunnelv1.TunnelFrame) (*tunnelv1.TunnelFrame, error) {
	streamId := frame.GetStreamId()
	requestId := frame.GetRequestId()
	switch payload := frame.GetPayload().(type) {
	case *tunnelv1.TunnelFrame_ListWorkspacesReq:
		items, err := runtime.ListWorkspaces(ctx)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_ListWorkspacesResp{ListWorkspacesResp: &runtimev1.ListWorkspacesResp{Items: workspacesToProto(items)}}), nil
	case *tunnelv1.TunnelFrame_WorkspaceTreeReq:
		items, err := runtime.WorkspaceTree(ctx)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_WorkspaceTreeResp{WorkspaceTreeResp: &runtimev1.WorkspaceTreeResp{Items: workspaceTreeToProto(items)}}), nil
	case *tunnelv1.TunnelFrame_UpdateWorkspaceOrderReq:
		items, err := runtime.UpdateWorkspaceOrder(ctx, payload.UpdateWorkspaceOrderReq.GetWorkspaceIds())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_UpdateWorkspaceOrderResp{UpdateWorkspaceOrderResp: &runtimev1.UpdateWorkspaceOrderResp{Items: workspacesToProto(items)}}), nil
	case *tunnelv1.TunnelFrame_DeleteWorkspaceReq:
		if err := runtime.DeleteWorkspace(ctx, payload.DeleteWorkspaceReq.GetWorkspaceId()); err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_DeleteWorkspaceResp{DeleteWorkspaceResp: &runtimev1.DeleteWorkspaceResp{}}), nil
	case *tunnelv1.TunnelFrame_WorkspaceSessionsReq:
		workspaceId := payload.WorkspaceSessionsReq.GetWorkspaceId()
		items, err := runtime.ListSessionsByWorkspaceId(ctx, workspaceId)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_WorkspaceSessionsResp{WorkspaceSessionsResp: &runtimev1.WorkspaceSessionsResp{Items: workspaceSessionsToProto(workspaceId, items)}}), nil
	case *tunnelv1.TunnelFrame_UpdateSessionOrderReq:
		workspaceId := payload.UpdateSessionOrderReq.GetWorkspaceId()
		items, err := runtime.UpdateSessionOrder(ctx, workspaceId, payload.UpdateSessionOrderReq.GetSessionIds())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_UpdateSessionOrderResp{UpdateSessionOrderResp: &runtimev1.UpdateSessionOrderResp{Items: workspaceSessionsToProto(workspaceId, items)}}), nil
	case *tunnelv1.TunnelFrame_CreateSessionReq:
		result, err := runtime.CreateSession(ctx, terminalapp.CreateSessionReq{WorkspaceId: payload.CreateSessionReq.GetWorkspaceId(), Name: payload.CreateSessionReq.GetName(), Cwd: payload.CreateSessionReq.GetCwd(), Command: payload.CreateSessionReq.GetCommand(), Cols: int(payload.CreateSessionReq.GetCols()), Rows: int(payload.CreateSessionReq.GetRows())})
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_CreateSessionResp{CreateSessionResp: createSessionToProto(result)}), nil
	case *tunnelv1.TunnelFrame_RerunSessionReq:
		result, err := runtime.RerunSession(ctx, payload.RerunSessionReq.GetWorkspaceId(), payload.RerunSessionReq.GetSessionId(), terminalapp.RerunSessionReq{Cols: int(payload.RerunSessionReq.GetCols()), Rows: int(payload.RerunSessionReq.GetRows())})
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_RerunSessionResp{RerunSessionResp: createSessionToProto(result)}), nil
	case *tunnelv1.TunnelFrame_GetSessionReq:
		session, err := runtime.GetSession(ctx, payload.GetSessionReq.GetWorkspaceId(), payload.GetSessionReq.GetSessionId())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_GetSessionResp{GetSessionResp: &runtimev1.GetSessionResp{Session: sessionToProto(session)}}), nil
	case *tunnelv1.TunnelFrame_UpdateSessionReq:
		request := terminalapp.UpdateSessionReq{}
		if payload.UpdateSessionReq.Name != nil {
			request.Name = payload.UpdateSessionReq.GetName()
		}
		session, err := runtime.UpdateSession(ctx, payload.UpdateSessionReq.GetWorkspaceId(), payload.UpdateSessionReq.GetSessionId(), request)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_UpdateSessionResp{UpdateSessionResp: &runtimev1.UpdateSessionResp{Session: sessionToProto(session)}}), nil
	case *tunnelv1.TunnelFrame_DeleteSessionReq:
		if err := runtime.DeleteSession(ctx, payload.DeleteSessionReq.GetWorkspaceId(), payload.DeleteSessionReq.GetSessionId()); err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_DeleteSessionResp{DeleteSessionResp: &runtimev1.DeleteSessionResp{}}), nil
	case *tunnelv1.TunnelFrame_CloseSessionReq:
		session, err := runtime.CloseSession(ctx, payload.CloseSessionReq.GetWorkspaceId(), payload.CloseSessionReq.GetSessionId())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_CloseSessionResp{CloseSessionResp: &runtimev1.CloseSessionResp{Session: sessionToProto(session)}}), nil
	case *tunnelv1.TunnelFrame_ReadHistoryReq:
		data, err := runtime.ReadHistory(ctx, payload.ReadHistoryReq.GetWorkspaceId(), payload.ReadHistoryReq.GetSessionId())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &tunnelv1.TunnelFrame_ReadHistoryResp{ReadHistoryResp: &runtimev1.ReadHistoryResp{Text: string(data)}}), nil
	default:
		return nil, fmt.Errorf("unsupported runtime payload %T", frame.GetPayload())
	}
}

func runtimeResponse(streamId string, requestId string, payload any) *tunnelv1.TunnelFrame {
	frame := &tunnelv1.TunnelFrame{StreamId: streamId, RequestId: requestId}
	switch value := payload.(type) {
	case *tunnelv1.TunnelFrame_ListWorkspacesResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_WorkspaceTreeResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_WorkspaceSessionsResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_CreateSessionResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_GetSessionResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_RerunSessionResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_UpdateSessionResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_CloseSessionResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_DeleteSessionResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_ReadHistoryResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_UpdateWorkspaceOrderResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_UpdateSessionOrderResp:
		frame.Payload = value
	case *tunnelv1.TunnelFrame_DeleteWorkspaceResp:
		frame.Payload = value
	}
	return frame
}

func isRuntimeRequest(frame *tunnelv1.TunnelFrame) bool {
	switch frame.GetPayload().(type) {
	case *tunnelv1.TunnelFrame_ListWorkspacesReq,
		*tunnelv1.TunnelFrame_WorkspaceTreeReq,
		*tunnelv1.TunnelFrame_WorkspaceSessionsReq,
		*tunnelv1.TunnelFrame_CreateSessionReq,
		*tunnelv1.TunnelFrame_GetSessionReq,
		*tunnelv1.TunnelFrame_RerunSessionReq,
		*tunnelv1.TunnelFrame_UpdateSessionReq,
		*tunnelv1.TunnelFrame_CloseSessionReq,
		*tunnelv1.TunnelFrame_DeleteSessionReq,
		*tunnelv1.TunnelFrame_ReadHistoryReq,
		*tunnelv1.TunnelFrame_UpdateWorkspaceOrderReq,
		*tunnelv1.TunnelFrame_UpdateSessionOrderReq,
		*tunnelv1.TunnelFrame_DeleteWorkspaceReq:
		return true
	default:
		return false
	}
}

func (c *Client) handleTerminal(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame *tunnelv1.TunnelFrame) {
	payload := frame.GetTerminalAttach()
	if payload == nil {
		_ = writeTerminalError(ctx, conn, writeMu, frame.GetStreamId(), frame.GetRequestId(), "invalid terminal attach")
		return
	}
	attrs := []any{"stream_id", frame.GetStreamId(), "workspace_id", payload.GetWorkspaceId(), "session_id", payload.GetSessionId(), "request_id", frame.GetRequestId()}
	if c.config.Runtime == nil {
		err := fmt.Errorf("runtime access unavailable")
		c.logWarn("agent terminal attach failed", append(attrs, "error", err)...)
		_ = writeTerminalError(ctx, conn, writeMu, frame.GetStreamId(), frame.GetRequestId(), err.Error())
		return
	}
	stream, err := c.config.Runtime.Attach(ctx, payload.GetWorkspaceId(), payload.GetSessionId())
	if err != nil {
		c.logWarn("agent terminal attach failed", append(attrs, "error", err)...)
		_ = writeTerminalError(ctx, conn, writeMu, frame.GetStreamId(), frame.GetRequestId(), err.Error())
		return
	}
	c.logInfo("agent terminal attached", attrs...)
	defer func() {
		stream.Detach("agent_detached")
		c.logInfo("agent terminal detached", attrs...)
	}()
	if payload.GetCols() > 0 && payload.GetRows() > 0 {
		if err := terminalproto.ValidateSize(int(payload.GetCols()), int(payload.GetRows())); err != nil {
			c.logWarn("agent terminal attach resize invalid", append(attrs, "cols", payload.GetCols(), "rows", payload.GetRows(), "error", err)...)
			_ = writeTerminalError(ctx, conn, writeMu, frame.GetStreamId(), frame.GetRequestId(), err.Error())
			return
		}
		if err := stream.Resize(int(payload.GetCols()), int(payload.GetRows())); err != nil {
			c.logWarn("agent terminal attach resize failed", append(attrs, "cols", payload.GetCols(), "rows", payload.GetRows(), "error", err)...)
			_ = writeTerminalError(ctx, conn, writeMu, frame.GetStreamId(), frame.GetRequestId(), err.Error())
			return
		}
	}
	inbound := make(chan *tunnelv1.TunnelFrame, 16)
	c.addTerminal(frame.GetStreamId(), inbound)
	defer c.removeTerminal(frame.GetStreamId())
	for {
		select {
		case <-ctx.Done():
			return
		case inboundFrame := <-inbound:
			switch payload := inboundFrame.GetPayload().(type) {
			case *tunnelv1.TunnelFrame_TerminalInput:
				if err := stream.WriteInput(payload.TerminalInput.GetData()); err != nil {
					c.logWarn("agent terminal input write failed", append(attrs, "bytes", len(payload.TerminalInput.GetData()), "error", err)...)
				}
			case *tunnelv1.TunnelFrame_TerminalResize:
				cols := int(payload.TerminalResize.GetCols())
				rows := int(payload.TerminalResize.GetRows())
				if err := terminalproto.ValidateSize(cols, rows); err != nil {
					c.logWarn("agent terminal resize invalid", append(attrs, "cols", cols, "rows", rows, "error", err)...)
					continue
				}
				if err := stream.Resize(cols, rows); err != nil {
					c.logWarn("agent terminal resize failed", append(attrs, "cols", cols, "rows", rows, "error", err)...)
				}
			case *tunnelv1.TunnelFrame_Close:
				return
			}
		case outbound, ok := <-stream.Outbound():
			if !ok {
				closed := &tunnelv1.TunnelFrame{StreamId: frame.GetStreamId(), Payload: &tunnelv1.TunnelFrame_TerminalClosed{TerminalClosed: &tunnelv1.TerminalClosed{}}}
				_ = writeTunnelFrame(ctx, conn, writeMu, closed)
				return
			}
			data := outboundData(outbound)
			output := &tunnelv1.TunnelFrame{StreamId: frame.GetStreamId(), Payload: &tunnelv1.TunnelFrame_TerminalOutput{TerminalOutput: &tunnelv1.TerminalOutput{Data: data}}}
			_ = writeTunnelFrame(ctx, conn, writeMu, output)
		}
	}
}

func (c *Client) dispatchTerminal(frame *tunnelv1.TunnelFrame) bool {
	c.mu.Lock()
	ch := c.terms[frame.GetStreamId()]
	c.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- frame:
	default:
		c.logWarn("agent terminal frame dropped", "stream_id", frame.GetStreamId(), "payload", fmt.Sprintf("%T", frame.GetPayload()))
	}
	return true
}

func (c *Client) addTerminal(streamId string, ch chan *tunnelv1.TunnelFrame) {
	c.mu.Lock()
	c.terms[streamId] = ch
	c.mu.Unlock()
}

func (c *Client) removeTerminal(streamId string) {
	c.mu.Lock()
	delete(c.terms, streamId)
	c.mu.Unlock()
}

func writeTerminalError(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, streamId string, requestId string, message string) error {
	return writeTunnelFrame(ctx, conn, writeMu, tunnel.ErrorFrame(streamId, requestId, "terminal_stream_error", message))
}

func writeTunnelFrame(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame *tunnelv1.TunnelFrame) error {
	data, err := tunnel.MarshalFrame(frame)
	if err != nil {
		return err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	return conn.Write(ctx, websocket.MessageText, data)
}

func outboundData(outbound terminalapp.Outbound) []byte {
	if outbound.Kind == terminalapp.OutboundBinary {
		return outbound.Binary
	}
	return []byte(outbound.Text.Message)
}

func workspacesToProto(items []terminalapp.WorkspaceSummary) []*runtimev1.Workspace {
	out := make([]*runtimev1.Workspace, 0, len(items))
	for _, item := range items {
		out = append(out, workspaceToProto(item))
	}
	return out
}

func workspaceToProto(item terminalapp.WorkspaceSummary) *runtimev1.Workspace {
	return &runtimev1.Workspace{Id: item.Id, Name: item.Name, Path: item.Path, UpdatedAt: timestamp(item.UpdatedAt)}
}

func workspaceTreeToProto(items []terminalapp.WorkspaceTreeNode) []*runtimev1.WorkspaceTreeNode {
	out := make([]*runtimev1.WorkspaceTreeNode, 0, len(items))
	for _, item := range items {
		out = append(out, &runtimev1.WorkspaceTreeNode{Id: item.Id, Name: item.Name, Path: item.Path, UpdatedAt: timestamp(item.UpdatedAt), Children: workspaceSessionsToProto(item.Id, item.Children)})
	}
	return out
}

func workspaceSessionsToProto(workspaceId string, items []terminalapp.WorkspaceSessionSummary) []*runtimev1.SessionSummary {
	out := make([]*runtimev1.SessionSummary, 0, len(items))
	for _, item := range items {
		out = append(out, workspaceSessionToProto(workspaceId, item))
	}
	return out
}

func workspaceSessionToProto(workspaceId string, item terminalapp.WorkspaceSessionSummary) *runtimev1.SessionSummary {
	var exitCode *int32
	if item.ExitCode != nil {
		value := int32(*item.ExitCode)
		exitCode = &value
	}
	return &runtimev1.SessionSummary{Id: item.Id, WorkspaceId: workspaceId, Name: item.Name, Command: item.Command, Cwd: item.Cwd, LifecycleState: string(item.LifecycleState), AttachmentState: string(item.AttachmentState), ExitCode: exitCode, UpdatedAt: timestamp(item.UpdatedAt)}
}

func sessionToProto(item terminalapp.SessionSummary) *runtimev1.SessionSummary {
	var exitCode *int32
	if item.ExitCode != nil {
		value := int32(*item.ExitCode)
		exitCode = &value
	}
	return &runtimev1.SessionSummary{Id: item.Id, WorkspaceId: item.WorkspaceId, Name: item.Name, Command: item.Command, Cwd: item.Cwd, LifecycleState: string(item.LifecycleState), AttachmentState: string(item.AttachmentState), ExitCode: exitCode, UpdatedAt: timestamp(item.UpdatedAt)}
}

func createSessionToProto(item terminalapp.CreateSessionResp) *runtimev1.CreateSessionResp {
	return &runtimev1.CreateSessionResp{SessionId: item.SessionId, WorkspaceId: item.WorkspaceId, State: item.State}
}

func timestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func (c *Client) tunnelHeader(device Device) (http.Header, error) {
	privateKey, err := LoadDevicePrivateKey(c.config.StateDir, device.Id)
	if err != nil {
		return nil, err
	}
	return tunnel.SignedTunnelHeader(http.MethodGet, "/cloud-api/agent/tunnel", c.config.ConnectUrl, device.Id, privateKey, time.Now(), "")
}

func (c *Client) Config() Config {
	return c.config
}

func (c *Client) Device() Device {
	return c.device
}

func tunnelUrl(base string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/cloud-api/agent/tunnel"
	}
	return parsed.String()
}
