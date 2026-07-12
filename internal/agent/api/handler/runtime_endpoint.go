package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	terminalapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/terminal"
	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
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

func localRuntimeRequestFrame(method string, params any, requestId string) (*shared.TunnelFrame, error) {
	frame := &shared.TunnelFrame{RequestId: requestId}
	switch method {
	case "workspaces":
		frame.Payload = &shared.TunnelFrame_ListWorkspacesReq{ListWorkspacesReq: &agent.ListWorkspacesReq{}}
	case "workspace_tree":
		frame.Payload = &shared.TunnelFrame_WorkspaceTreeReq{WorkspaceTreeReq: &agent.WorkspaceTreeReq{}}
	case "workspace_order":
		req, ok := params.(*agent.UpdateWorkspaceOrderReq)
		if !ok {
			return nil, fmt.Errorf("workspace_order params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_UpdateWorkspaceOrderReq{UpdateWorkspaceOrderReq: req}
	case "delete_workspace":
		req, ok := params.(*agent.DeleteWorkspaceReq)
		if !ok {
			return nil, fmt.Errorf("delete_workspace params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_DeleteWorkspaceReq{DeleteWorkspaceReq: req}
	case "workspace_sessions":
		req, ok := params.(*agent.WorkspaceSessionsReq)
		if !ok {
			return nil, fmt.Errorf("workspace_sessions params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_WorkspaceSessionsReq{WorkspaceSessionsReq: req}
	case "session_order":
		req, ok := params.(*agent.WorkspaceSessionOrderReq)
		if !ok {
			return nil, fmt.Errorf("session_order params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_UpdateSessionOrderReq{UpdateSessionOrderReq: req}
	case "create_session":
		req, ok := params.(*agent.CreateSessionReq)
		if !ok {
			return nil, fmt.Errorf("create_session params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_CreateSessionReq{CreateSessionReq: req}
	case "rerun_session":
		req, ok := params.(*agent.RerunWorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("rerun_session params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_RerunSessionReq{RerunSessionReq: req}
	case "get_session":
		req, ok := params.(*agent.WorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("get_session params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_GetSessionReq{GetSessionReq: req}
	case "update_session":
		req, ok := params.(*agent.UpdateWorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("update_session params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_UpdateSessionReq{UpdateSessionReq: req}
	case "delete_session":
		req, ok := params.(*agent.WorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("delete_session params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_DeleteSessionReq{DeleteSessionReq: req}
	case "close_session":
		req, ok := params.(*agent.WorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("close_session params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_CloseSessionReq{CloseSessionReq: req}
	case "list_shortcuts":
		frame.Payload = &shared.TunnelFrame_ListShortcutsReq{ListShortcutsReq: &agent.ListShortcutsReq{}}
	case "create_shortcut":
		req, ok := params.(*agent.CreateShortcutReq)
		if !ok {
			return nil, fmt.Errorf("create_shortcut params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_CreateShortcutReq{CreateShortcutReq: req}
	case "update_shortcut":
		req, ok := params.(*agent.UpdateShortcutRequest)
		if !ok {
			return nil, fmt.Errorf("update_shortcut params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_UpdateShortcutReq{UpdateShortcutReq: req}
	case "update_shortcut_order":
		req, ok := params.(*agent.UpdateShortcutOrderReq)
		if !ok {
			return nil, fmt.Errorf("update_shortcut_order params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_UpdateShortcutOrderReq{UpdateShortcutOrderReq: req}
	case "delete_shortcut":
		req, ok := params.(*agent.DeleteShortcutReq)
		if !ok {
			return nil, fmt.Errorf("delete_shortcut params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_DeleteShortcutReq{DeleteShortcutReq: req}
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
		_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: sizeErr.Error()})
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
					_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: "bad_control", Message: err.Error()})
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
					_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
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
				err := conn.Write(r.Context(), websocket.MessageBinary, outbound.Binary)
				stream.MarkSent(outbound)
				if err != nil {
					return err
				}
				continue
			}
			data, err := terminalproto.EncodeServer(outbound.Text)
			if err != nil {
				stream.MarkSent(outbound)
				return err
			}
			err = conn.Write(r.Context(), websocket.MessageText, data)
			stream.MarkSent(outbound)
			if err != nil {
				return err
			}
		}
	}
}
