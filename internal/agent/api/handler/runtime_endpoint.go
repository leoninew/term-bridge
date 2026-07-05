package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	terminalapp "termbridge-go/internal/agent/application/task/terminal"
	agentapp "termbridge-go/internal/agent/application/user"
	runtimev1 "termbridge-go/internal/shared/dto/proto/termbridge/runtime/v1"
	tunnelv1 "termbridge-go/internal/shared/dto/proto/termbridge/tunnel/v1"
	terminalproto "termbridge-go/internal/shared/dto/protocol/terminal"
	"termbridge-go/internal/shared/dto/protocol/tunnel"
)

type runtimeEndpoint interface {
	JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error)
	History(ctx context.Context, workspaceId string, sessionId string, requestId string) (text string, offline bool, err error)
	Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error
	Available() bool
}

type localRuntimeEndpoint struct {
	runtime agentapp.RuntimeAccess
	handler *Handler
}

func (e localRuntimeEndpoint) Available() bool {
	return e.runtime != nil
}

func (e localRuntimeEndpoint) JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error) {
	request, err := localRuntimeRequestFrame(method, params, requestId)
	if err != nil {
		return nil, err
	}
	request.StreamId = "local-req"
	response, err := agentapp.HandleRuntimeRequest(ctx, e.runtime, request)
	if err != nil {
		return nil, err
	}
	return tunnel.ResponseJSON(response)
}

func (e localRuntimeEndpoint) History(ctx context.Context, workspaceId string, sessionId string, _ string) (string, bool, error) {
	data, err := e.runtime.ReadHistory(ctx, workspaceId, sessionId)
	return string(data), false, err
}

func (e localRuntimeEndpoint) Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error {
	return e.handler.bridgeTerminalStream(w, r, e.runtime, workspaceId, sessionId)
}

func localRuntimeRequestFrame(method string, params any, requestId string) (*tunnelv1.TunnelFrame, error) {
	frame := &tunnelv1.TunnelFrame{RequestId: requestId}
	switch method {
	case "workspaces":
		frame.Payload = &tunnelv1.TunnelFrame_ListWorkspacesReq{ListWorkspacesReq: &runtimev1.ListWorkspacesReq{}}
	case "workspace_tree":
		frame.Payload = &tunnelv1.TunnelFrame_WorkspaceTreeReq{WorkspaceTreeReq: &runtimev1.WorkspaceTreeReq{}}
	case "workspace_order":
		req, ok := params.(terminalapp.UpdateWorkspaceOrderReq)
		if !ok {
			return nil, fmt.Errorf("workspace_order params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_UpdateWorkspaceOrderReq{UpdateWorkspaceOrderReq: &runtimev1.UpdateWorkspaceOrderReq{WorkspaceIds: req.WorkspaceIds}}
	case "delete_workspace":
		req, ok := params.(terminalapp.DeleteWorkspaceReq)
		if !ok {
			return nil, fmt.Errorf("delete_workspace params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_DeleteWorkspaceReq{DeleteWorkspaceReq: &runtimev1.DeleteWorkspaceReq{WorkspaceId: req.WorkspaceId}}
	case "workspace_sessions":
		req, ok := params.(terminalapp.WorkspaceSessionsReq)
		if !ok {
			return nil, fmt.Errorf("workspace_sessions params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_WorkspaceSessionsReq{WorkspaceSessionsReq: &runtimev1.WorkspaceSessionsReq{WorkspaceId: req.WorkspaceId}}
	case "session_order":
		req, ok := params.(terminalapp.WorkspaceSessionOrderReq)
		if !ok {
			return nil, fmt.Errorf("session_order params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_UpdateSessionOrderReq{UpdateSessionOrderReq: &runtimev1.WorkspaceSessionOrderReq{WorkspaceId: req.WorkspaceId, SessionIds: req.SessionIds}}
	case "create_session":
		req, ok := params.(terminalapp.CreateSessionReq)
		if !ok {
			return nil, fmt.Errorf("create_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_CreateSessionReq{CreateSessionReq: &runtimev1.CreateSessionReq{WorkspaceId: req.WorkspaceId, Name: req.Name, Cwd: req.Cwd, Command: req.Command, Cols: int32(req.Cols), Rows: int32(req.Rows)}}
	case "rerun_session":
		req, ok := params.(terminalapp.RerunWorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("rerun_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_RerunSessionReq{RerunSessionReq: &runtimev1.RerunWorkspaceSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId, Request: &runtimev1.RerunSessionReq{Cols: int32(req.Request.Cols), Rows: int32(req.Request.Rows)}}}
	case "get_session":
		req, ok := params.(terminalapp.WorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("get_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_GetSessionReq{GetSessionReq: &runtimev1.WorkspaceSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId}}
	case "update_session":
		req, ok := params.(terminalapp.UpdateWorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("update_session params have type %T", params)
		}
		name := req.Request.Name
		frame.Payload = &tunnelv1.TunnelFrame_UpdateSessionReq{UpdateSessionReq: &runtimev1.UpdateWorkspaceSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId, Request: &runtimev1.UpdateSessionReq{Name: &name}}}
	case "delete_session":
		req, ok := params.(terminalapp.WorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("delete_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_DeleteSessionReq{DeleteSessionReq: &runtimev1.WorkspaceSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId}}
	case "close_session":
		req, ok := params.(terminalapp.WorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("close_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_CloseSessionReq{CloseSessionReq: &runtimev1.WorkspaceSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId}}
	default:
		return nil, fmt.Errorf("unknown runtime method %q", method)
	}
	return frame, nil
}

func (s *Handler) bridgeTerminalStream(w http.ResponseWriter, r *http.Request, runtime agentapp.RuntimeAccess, workspaceId string, sessionId string) error {
	writerKey := sessionScopeKey(workspaceId, sessionId)
	s.writerMu.Lock()
	if _, exists := s.writers[writerKey]; exists {
		s.writerMu.Unlock()
		s.writeAPIError(w, r, http.StatusConflict, errorCodeConflict, errorMessageConflict, nil)
		return nil
	}
	streamId := "local-term-" + fmt.Sprint(time.Now().UnixNano())
	s.writers[writerKey] = streamId
	s.writerMu.Unlock()
	defer func() {
		s.writerMu.Lock()
		delete(s.writers, writerKey)
		s.writerMu.Unlock()
	}()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{terminalproto.Subprotocol}, OriginPatterns: s.originPatterns(r)})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	cols, rows, hasAttachSize, sizeErr := terminalAttachSizeFromQuery(r)
	if sizeErr != nil {
		s.config.Logger.Warn("terminal attach size invalid", "workspace_id", workspaceId, "session_id", sessionId, "error", sizeErr)
		_ = writeTerminalControl(conn, &terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: sizeErr.Error()})
		return nil
	}

	stream, err := runtime.Attach(r.Context(), workspaceId, sessionId)
	if err != nil {
		return err
	}
	defer stream.Detach("agent_detached")
	if hasAttachSize {
		if err := stream.Resize(cols, rows); err != nil {
			return err
		}
	}

	done := make(chan struct{})
	var closeDone sync.Once
	stop := func() { closeDone.Do(func() { close(done) }) }

	go func() {
		defer stop()
		for {
			messageType, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			switch messageType {
			case websocket.MessageText:
				message, err := terminalproto.DecodeClient(data)
				if err != nil {
					_ = writeTerminalControl(conn, &terminalproto.ServerMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: err.Error()})
					continue
				}
				switch message.Type {
				case terminalproto.TypeHello:
					continue
				case terminalproto.TypeResize:
					cols := int(message.Cols)
					rows := int(message.Rows)
					if err := stream.Resize(cols, rows); err != nil {
						s.config.Logger.Warn("terminal resize failed", "workspace_id", workspaceId, "session_id", sessionId, "cols", cols, "rows", rows, "error", err)
					}
				case terminalproto.TypeDetach:
					stream.Detach("browser_detached")
					return
				case terminalproto.TypePing:
					_ = writeTerminalControl(conn, &terminalproto.ServerMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
				}
			case websocket.MessageBinary:
				if err := stream.WriteInput(data); err != nil {
					s.config.Logger.Warn("terminal input write failed", "workspace_id", workspaceId, "session_id", sessionId, "bytes", len(data), "error", err)
				}
			}
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-done:
			return nil
		case outbound, ok := <-stream.Outbound():
			if !ok {
				_ = conn.Close(websocket.StatusNormalClosure, "terminal closed")
				return nil
			}
			if outbound.Kind == terminalapp.OutboundBinary {
				if err := conn.Write(r.Context(), websocket.MessageBinary, outbound.Binary); err != nil {
					return err
				}
				continue
			}
			data, err := terminalproto.EncodeServer(outbound.Text)
			if err != nil {
				return err
			}
			if err := conn.Write(r.Context(), websocket.MessageText, data); err != nil {
				return err
			}
		}
	}
}
