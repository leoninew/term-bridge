package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/types/known/structpb"

	terminalapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/terminal"
	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
	tunnel "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
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
	terms  map[string]chan *shared.TunnelFrame
}

func (c *Client) SetDevice(device Device) {
	c.device = device
}

func New(config Config) *Client {
	return &Client{config: config, terms: map[string]chan *shared.TunnelFrame{}}
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
	hello := &shared.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &shared.TunnelFrame_Hello{Hello: &shared.Hello{DeviceId: device.Id, DeviceName: device.Name, ProtocolVersion: tunnel.ProtocolVersion}}}
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
	if ack.GetHelloAck().GetProtocolVersion() != tunnel.ProtocolVersion {
		return websocket.CloseError{Code: websocket.StatusPolicyViolation, Reason: "unsupported tunnel protocol version"}
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
		case *shared.TunnelFrame_TerminalAttach:
			go c.handleTerminal(ctx, conn, &writeMu, frame)
		case *shared.TunnelFrame_Ping:
			pong := &shared.TunnelFrame{StreamId: tunnel.ControlStreamID, Payload: &shared.TunnelFrame_Pong{Pong: &shared.Pong{Nonce: frame.GetPing().GetNonce()}}}
			_ = writeTunnelFrame(ctx, conn, &writeMu, pong)
		default:
			if isRuntimeRequest(frame) {
				go c.handleRequest(ctx, conn, &writeMu, frame)
			}
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame *shared.TunnelFrame) {
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
		_ = writeTunnelFrame(ctx, conn, writeMu, runtimeErrorFrame(frame.GetStreamId(), frame.GetRequestId(), err))
		return
	}
	c.logInfo("agent tunnel request handled", attrs...)
	_ = writeTunnelFrame(ctx, conn, writeMu, response)
}

func (c *Client) handleRuntimeRequest(ctx context.Context, frame *shared.TunnelFrame) (*shared.TunnelFrame, error) {
	return HandleRuntimeRequest(ctx, c.config.Runtime, frame)
}

func HandleRuntimeRequest(ctx context.Context, runtimeAccess RuntimeAccess, frame *shared.TunnelFrame) (*shared.TunnelFrame, error) {
	streamId := frame.GetStreamId()
	requestId := frame.GetRequestId()
	switch payload := frame.GetPayload().(type) {
	case *shared.TunnelFrame_ListWorkspacesReq:
		items, err := runtimeAccess.ListWorkspaces(ctx)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_ListWorkspacesResp{ListWorkspacesResp: &agent.ListWorkspacesResp{Items: items}}), nil
	case *shared.TunnelFrame_WorkspaceTreeReq:
		items, err := runtimeAccess.WorkspaceTree(ctx)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_WorkspaceTreeResp{WorkspaceTreeResp: &agent.WorkspaceTreeResp{Items: items}}), nil
	case *shared.TunnelFrame_UpdateWorkspaceOrderReq:
		items, err := runtimeAccess.UpdateWorkspaceOrder(ctx, payload.UpdateWorkspaceOrderReq.GetWorkspaceIds())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_UpdateWorkspaceOrderResp{UpdateWorkspaceOrderResp: &agent.UpdateWorkspaceOrderResp{Items: items}}), nil
	case *shared.TunnelFrame_DeleteWorkspaceReq:
		if err := runtimeAccess.DeleteWorkspace(ctx, payload.DeleteWorkspaceReq.GetWorkspaceId()); err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_DeleteWorkspaceResp{DeleteWorkspaceResp: &agent.DeleteWorkspaceResp{}}), nil
	case *shared.TunnelFrame_WorkspaceSessionsReq:
		workspaceId := payload.WorkspaceSessionsReq.GetWorkspaceId()
		items, err := runtimeAccess.ListSessionsByWorkspaceId(ctx, workspaceId)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_WorkspaceSessionsResp{WorkspaceSessionsResp: &agent.WorkspaceSessionsResp{Items: items}}), nil
	case *shared.TunnelFrame_UpdateSessionOrderReq:
		workspaceId := payload.UpdateSessionOrderReq.GetWorkspaceId()
		items, err := runtimeAccess.UpdateSessionOrder(ctx, workspaceId, payload.UpdateSessionOrderReq.GetSessionIds())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_UpdateSessionOrderResp{UpdateSessionOrderResp: &agent.UpdateSessionOrderResp{Items: items}}), nil
	case *shared.TunnelFrame_CreateSessionReq:
		result, err := runtimeAccess.CreateSession(ctx, payload.CreateSessionReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_CreateSessionResp{CreateSessionResp: result}), nil
	case *shared.TunnelFrame_RerunSessionReq:
		result, err := runtimeAccess.RerunSession(ctx, payload.RerunSessionReq.GetWorkspaceId(), payload.RerunSessionReq.GetSessionId(), payload.RerunSessionReq.GetRequest())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_RerunSessionResp{RerunSessionResp: result}), nil
	case *shared.TunnelFrame_GetSessionReq:
		session, err := runtimeAccess.GetSession(ctx, payload.GetSessionReq.GetWorkspaceId(), payload.GetSessionReq.GetSessionId())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_GetSessionResp{GetSessionResp: &agent.GetSessionResp{Session: session}}), nil
	case *shared.TunnelFrame_UpdateSessionReq:
		session, err := runtimeAccess.UpdateSession(ctx, payload.UpdateSessionReq.GetWorkspaceId(), payload.UpdateSessionReq.GetSessionId(), payload.UpdateSessionReq.GetRequest())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_UpdateSessionResp{UpdateSessionResp: &agent.UpdateSessionResp{Session: session}}), nil
	case *shared.TunnelFrame_DeleteSessionReq:
		if err := runtimeAccess.DeleteSession(ctx, payload.DeleteSessionReq.GetWorkspaceId(), payload.DeleteSessionReq.GetSessionId()); err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_DeleteSessionResp{DeleteSessionResp: &agent.DeleteSessionResp{}}), nil
	case *shared.TunnelFrame_CloseSessionReq:
		session, err := runtimeAccess.CloseSession(ctx, payload.CloseSessionReq.GetWorkspaceId(), payload.CloseSessionReq.GetSessionId())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_CloseSessionResp{CloseSessionResp: &agent.CloseSessionResp{Session: session}}), nil
	case *shared.TunnelFrame_ReadHistoryReq:
		data, err := runtimeAccess.ReadHistory(ctx, payload.ReadHistoryReq.GetWorkspaceId(), payload.ReadHistoryReq.GetSessionId())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_ReadHistoryResp{ReadHistoryResp: &agent.ReadHistoryResp{Text: string(data)}}), nil
	case *shared.TunnelFrame_ListShortcutsReq:
		items, err := runtimeAccess.ListShortcuts(ctx)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_ListShortcutsResp{ListShortcutsResp: &agent.ListShortcutsResp{Items: items}}), nil
	case *shared.TunnelFrame_CreateShortcutReq:
		value, err := runtimeAccess.CreateShortcut(ctx, payload.CreateShortcutReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_CreateShortcutResp{CreateShortcutResp: &agent.CreateShortcutResp{Shortcut: value}}), nil
	case *shared.TunnelFrame_UpdateShortcutReq:
		value, err := runtimeAccess.UpdateShortcut(ctx, payload.UpdateShortcutReq.GetShortcutId(), payload.UpdateShortcutReq.GetRequest())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_UpdateShortcutResp{UpdateShortcutResp: &agent.UpdateShortcutResp{Shortcut: value}}), nil
	case *shared.TunnelFrame_UpdateShortcutOrderReq:
		items, err := runtimeAccess.UpdateShortcutOrder(ctx, payload.UpdateShortcutOrderReq.GetShortcutIds())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_UpdateShortcutOrderResp{UpdateShortcutOrderResp: &agent.UpdateShortcutOrderResp{Items: items}}), nil
	case *shared.TunnelFrame_DeleteShortcutReq:
		if err := runtimeAccess.DeleteShortcut(ctx, payload.DeleteShortcutReq.GetShortcutId()); err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_DeleteShortcutResp{DeleteShortcutResp: &agent.DeleteShortcutResp{}}), nil
	case *shared.TunnelFrame_ListFilesReq:
		result, err := runtimeAccess.ListFiles(ctx, payload.ListFilesReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_ListFilesResp{ListFilesResp: result}), nil
	case *shared.TunnelFrame_ReadFileReq:
		result, err := runtimeAccess.ReadFile(ctx, payload.ReadFileReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_ReadFileResp{ReadFileResp: result}), nil
	case *shared.TunnelFrame_CreateFileReq:
		result, err := runtimeAccess.CreateFile(ctx, payload.CreateFileReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_CreateFileResp{CreateFileResp: result}), nil
	case *shared.TunnelFrame_CreateDirectoryReq:
		result, err := runtimeAccess.CreateDirectory(ctx, payload.CreateDirectoryReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_CreateDirectoryResp{CreateDirectoryResp: result}), nil
	case *shared.TunnelFrame_WriteFileReq:
		result, err := runtimeAccess.WriteFile(ctx, payload.WriteFileReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_WriteFileResp{WriteFileResp: result}), nil
	case *shared.TunnelFrame_RenameEntryReq:
		result, err := runtimeAccess.RenameEntry(ctx, payload.RenameEntryReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_RenameEntryResp{RenameEntryResp: result}), nil
	case *shared.TunnelFrame_MoveEntryReq:
		result, err := runtimeAccess.MoveEntry(ctx, payload.MoveEntryReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_MoveEntryResp{MoveEntryResp: result}), nil
	case *shared.TunnelFrame_DeleteEntryReq:
		result, err := runtimeAccess.DeleteEntry(ctx, payload.DeleteEntryReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_DeleteEntryResp{DeleteEntryResp: result}), nil
	case *shared.TunnelFrame_GitStatusReq:
		result, err := runtimeAccess.GitStatus(ctx, payload.GitStatusReq.GetWorkspaceId())
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_GitStatusResp{GitStatusResp: result}), nil
	case *shared.TunnelFrame_GitDiffReq:
		result, err := runtimeAccess.GitDiff(ctx, payload.GitDiffReq)
		if err != nil {
			return nil, err
		}
		return runtimeResponse(streamId, requestId, &shared.TunnelFrame_GitDiffResp{GitDiffResp: result}), nil
	default:
		return nil, fmt.Errorf("unsupported runtime payload %T", frame.GetPayload())
	}
}

type RuntimeError interface {
	error
	RuntimeErrorCode() string
}

func runtimeErrorFrame(streamId string, requestId string, err error) *shared.TunnelFrame {
	response := &shared.ErrorResp{Code: "runtime_error", Error: "Runtime request failed.", RequestId: requestId}
	var typed RuntimeError
	if errors.As(err, &typed) {
		response.Code = typed.RuntimeErrorCode()
	}
	response.Details = fileConflictDetails(err)
	return &shared.TunnelFrame{StreamId: streamId, RequestId: requestId, Payload: &shared.TunnelFrame_Error{Error: response}}
}

func fileConflictDetails(err error) *structpb.Struct {
	var fileErr *filemodel.Error
	if !errors.As(err, &fileErr) || fileErr.Code != "revision_conflict" || fileErr.Entry == nil {
		return nil
	}
	entry := fileErr.Entry
	details, detailErr := structpb.NewStruct(map[string]any{
		"type": "revision_conflict",
		"current_entry": map[string]any{
			"path":     entry.Path.String(),
			"name":     entry.Name,
			"kind":     string(entry.Kind),
			"size":     entry.Size,
			"revision": entry.Revision,
		},
	})
	if detailErr != nil {
		return nil
	}
	return details
}

func runtimeResponse(streamId string, requestId string, payload any) *shared.TunnelFrame {
	frame := &shared.TunnelFrame{StreamId: streamId, RequestId: requestId}
	switch value := payload.(type) {
	case *shared.TunnelFrame_ListWorkspacesResp:
		frame.Payload = value
	case *shared.TunnelFrame_WorkspaceTreeResp:
		frame.Payload = value
	case *shared.TunnelFrame_WorkspaceSessionsResp:
		frame.Payload = value
	case *shared.TunnelFrame_CreateSessionResp:
		frame.Payload = value
	case *shared.TunnelFrame_GetSessionResp:
		frame.Payload = value
	case *shared.TunnelFrame_RerunSessionResp:
		frame.Payload = value
	case *shared.TunnelFrame_UpdateSessionResp:
		frame.Payload = value
	case *shared.TunnelFrame_CloseSessionResp:
		frame.Payload = value
	case *shared.TunnelFrame_DeleteSessionResp:
		frame.Payload = value
	case *shared.TunnelFrame_ReadHistoryResp:
		frame.Payload = value
	case *shared.TunnelFrame_UpdateWorkspaceOrderResp:
		frame.Payload = value
	case *shared.TunnelFrame_UpdateSessionOrderResp:
		frame.Payload = value
	case *shared.TunnelFrame_DeleteWorkspaceResp:
		frame.Payload = value
	case *shared.TunnelFrame_ListShortcutsResp:
		frame.Payload = value
	case *shared.TunnelFrame_CreateShortcutResp:
		frame.Payload = value
	case *shared.TunnelFrame_UpdateShortcutResp:
		frame.Payload = value
	case *shared.TunnelFrame_DeleteShortcutResp:
		frame.Payload = value
	case *shared.TunnelFrame_UpdateShortcutOrderResp:
		frame.Payload = value
	case *shared.TunnelFrame_ListFilesResp:
		frame.Payload = value
	case *shared.TunnelFrame_ReadFileResp:
		frame.Payload = value
	case *shared.TunnelFrame_CreateFileResp:
		frame.Payload = value
	case *shared.TunnelFrame_CreateDirectoryResp:
		frame.Payload = value
	case *shared.TunnelFrame_WriteFileResp:
		frame.Payload = value
	case *shared.TunnelFrame_RenameEntryResp:
		frame.Payload = value
	case *shared.TunnelFrame_MoveEntryResp:
		frame.Payload = value
	case *shared.TunnelFrame_DeleteEntryResp:
		frame.Payload = value
	case *shared.TunnelFrame_GitStatusResp:
		frame.Payload = value
	case *shared.TunnelFrame_GitDiffResp:
		frame.Payload = value
	}
	return frame
}

func isRuntimeRequest(frame *shared.TunnelFrame) bool {
	switch frame.GetPayload().(type) {
	case *shared.TunnelFrame_ListWorkspacesReq,
		*shared.TunnelFrame_WorkspaceTreeReq,
		*shared.TunnelFrame_WorkspaceSessionsReq,
		*shared.TunnelFrame_CreateSessionReq,
		*shared.TunnelFrame_GetSessionReq,
		*shared.TunnelFrame_RerunSessionReq,
		*shared.TunnelFrame_UpdateSessionReq,
		*shared.TunnelFrame_CloseSessionReq,
		*shared.TunnelFrame_DeleteSessionReq,
		*shared.TunnelFrame_ReadHistoryReq,
		*shared.TunnelFrame_UpdateWorkspaceOrderReq,
		*shared.TunnelFrame_UpdateSessionOrderReq,
		*shared.TunnelFrame_DeleteWorkspaceReq,
		*shared.TunnelFrame_ListShortcutsReq,
		*shared.TunnelFrame_CreateShortcutReq,
		*shared.TunnelFrame_UpdateShortcutReq,
		*shared.TunnelFrame_UpdateShortcutOrderReq,
		*shared.TunnelFrame_DeleteShortcutReq,
		*shared.TunnelFrame_ListFilesReq,
		*shared.TunnelFrame_ReadFileReq,
		*shared.TunnelFrame_CreateFileReq,
		*shared.TunnelFrame_CreateDirectoryReq,
		*shared.TunnelFrame_WriteFileReq,
		*shared.TunnelFrame_RenameEntryReq,
		*shared.TunnelFrame_MoveEntryReq,
		*shared.TunnelFrame_DeleteEntryReq,
		*shared.TunnelFrame_GitStatusReq,
		*shared.TunnelFrame_GitDiffReq:
		return true
	default:
		return false
	}
}

func (c *Client) handleTerminal(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame *shared.TunnelFrame) {
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
	inbound := make(chan *shared.TunnelFrame, 16)
	c.addTerminal(frame.GetStreamId(), inbound)
	defer c.removeTerminal(frame.GetStreamId())
	for {
		select {
		case <-ctx.Done():
			return
		case inboundFrame := <-inbound:
			switch payload := inboundFrame.GetPayload().(type) {
			case *shared.TunnelFrame_TerminalInput:
				if err := stream.WriteInput(payload.TerminalInput.GetData()); err != nil {
					c.logWarn("agent terminal input write failed", append(attrs, "bytes", len(payload.TerminalInput.GetData()), "error", err)...)
				}
			case *shared.TunnelFrame_TerminalResize:
				cols := int(payload.TerminalResize.GetCols())
				rows := int(payload.TerminalResize.GetRows())
				if err := terminalproto.ValidateSize(cols, rows); err != nil {
					c.logWarn("agent terminal resize invalid", append(attrs, "cols", cols, "rows", rows, "error", err)...)
					continue
				}
				if err := stream.Resize(cols, rows); err != nil {
					c.logWarn("agent terminal resize failed", append(attrs, "cols", cols, "rows", rows, "error", err)...)
				}
			case *shared.TunnelFrame_Close:
				return
			}
		case outbound, ok := <-stream.Outbound():
			if !ok {
				closed := &shared.TunnelFrame{StreamId: frame.GetStreamId(), Payload: &shared.TunnelFrame_TerminalClosed{TerminalClosed: &shared.TerminalClosed{}}}
				_ = writeTunnelFrame(ctx, conn, writeMu, closed)
				return
			}
			data := outboundData(outbound)
			output := &shared.TunnelFrame{StreamId: frame.GetStreamId(), Payload: &shared.TunnelFrame_TerminalOutput{TerminalOutput: &shared.TerminalOutput{Data: data}}}
			if err := writeTunnelFrame(ctx, conn, writeMu, output); err != nil {
				stream.MarkSent(outbound)
				return
			}
			stream.MarkSent(outbound)
		}
	}
}

func (c *Client) dispatchTerminal(frame *shared.TunnelFrame) bool {
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

func (c *Client) addTerminal(streamId string, ch chan *shared.TunnelFrame) {
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

func writeTunnelFrame(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, frame *shared.TunnelFrame) error {
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

func (c *Client) tunnelHeader(device Device) (http.Header, error) {
	privateKey, err := LoadDevicePrivateKey(c.config.StateDir, device.Id)
	if err != nil {
		return nil, err
	}
	return tunnel.SignedTunnelHeader(http.MethodGet, tunnelPath(c.config.ConnectUrl), c.config.ConnectUrl, device.Id, privateKey, time.Now(), "")
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
	parsed.Path = tunnelPath(base)
	return parsed.String()
}

func tunnelPath(base string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return "/agent/tunnel"
	}
	return strings.TrimRight(parsed.Path, "/") + "/agent/tunnel"
}
