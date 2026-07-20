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
	"gitee.com/leoninew/TermBridge-go/internal/shared/application/quota"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

type runtimeEndpoint interface {
	JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error)
	History(ctx context.Context, workspaceId string, sessionId string, requestId string) (text string, offline bool, err error)
	Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error
	WatchWorkspaceChanges(w http.ResponseWriter, r *http.Request, workspaceId string) error
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

func (e localRuntimeEndpoint) WatchWorkspaceChanges(w http.ResponseWriter, r *http.Request, workspaceId string) error {
	return e.handler.bridgeWorkspaceChanges(w, r, e.runtime, workspaceId)
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
	case "fs_stat":
		req, ok := params.(*agent.FsStatReq)
		if !ok {
			return nil, fmt.Errorf("fs_stat params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_FsStatReq{FsStatReq: req}
	case "fs_read_directory":
		req, ok := params.(*agent.FsReadDirectoryReq)
		if !ok {
			return nil, fmt.Errorf("fs_read_directory params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_FsReadDirectoryReq{FsReadDirectoryReq: req}
	case "fs_read_file":
		req, ok := params.(*agent.FsReadFileReq)
		if !ok {
			return nil, fmt.Errorf("fs_read_file params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_FsReadFileReq{FsReadFileReq: req}
	case "fs_write_file":
		req, ok := params.(*agent.FsWriteFileReq)
		if !ok {
			return nil, fmt.Errorf("fs_write_file params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_FsWriteFileReq{FsWriteFileReq: req}
	case "fs_create_directory":
		req, ok := params.(*agent.FsCreateDirectoryReq)
		if !ok {
			return nil, fmt.Errorf("fs_create_directory params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_FsCreateDirectoryReq{FsCreateDirectoryReq: req}
	case "fs_delete":
		req, ok := params.(*agent.FsDeleteReq)
		if !ok {
			return nil, fmt.Errorf("fs_delete params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_FsDeleteReq{FsDeleteReq: req}
	case "fs_rename":
		req, ok := params.(*agent.FsRenameReq)
		if !ok {
			return nil, fmt.Errorf("fs_rename params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_FsRenameReq{FsRenameReq: req}
	case "scm_status":
		req, ok := params.(*agent.ScmStatusReq)
		if !ok {
			return nil, fmt.Errorf("scm_status params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_ScmStatusReq{ScmStatusReq: req}
	case "scm_original_content":
		req, ok := params.(*agent.ScmOriginalContentReq)
		if !ok {
			return nil, fmt.Errorf("scm_original_content params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_ScmOriginalContentReq{ScmOriginalContentReq: req}
	case "scm_execute":
		req, ok := params.(*agent.ScmExecuteReq)
		if !ok {
			return nil, fmt.Errorf("scm_execute params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_ScmExecuteReq{ScmExecuteReq: req}
	case "scm_repository":
		req, ok := params.(*agent.ScmRepositoryReq)
		if !ok {
			return nil, fmt.Errorf("scm_repository params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_ScmRepositoryReq{ScmRepositoryReq: req}
	default:
		return nil, fmt.Errorf("unknown runtime method %q", method)
	}
	return frame, nil
}

func (s *Handler) bridgeTerminalStream(w http.ResponseWriter, r *http.Request, runtime agentapp.RuntimeAccess, workspaceId string, sessionId string) error {
	streamId := "local-term-" + fmt.Sprint(time.Now().UnixNano())

	subject := quota.LocalSubject
	if err := s.attachQuota.TryAcquire(subject, s.config.ConcurrentAttaches); err != nil {
		s.writeAPIError(w, r, http.StatusTooManyRequests, quota.CodeAttachExceeded, quota.MessageAttachExceeded, err)
		return nil
	}
	defer s.attachQuota.Release(subject)

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{terminalproto.Subprotocol}, OriginPatterns: s.originPatterns(r)})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	cols, rows, hasAttachSize, sizeErr := terminalAttachSizeFromQuery(r)
	if sizeErr != nil {
		s.config.Logger.Warn("terminal attach size invalid", "workspace_id", workspaceId, "session_id", sessionId, "error", sizeErr)
		_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: terminalproto.ErrorCodeBadControl, Message: terminalproto.ErrorMessageBadControl})
		return nil
	}

	stream, err := runtime.Attach(r.Context(), workspaceId, sessionId)
	if err != nil {
		return err
	}
	defer stream.Detach("agent_detached")
	if hasAttachSize && stream.IsController() {
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
				s.config.Logger.Info("terminal browser stream read ended", "workspace_id", workspaceId, "session_id", sessionId, "stream_id", streamId, "error", err)
				return
			}
			switch messageType {
			case websocket.MessageText:
				message, err := terminalproto.DecodeClient(data)
				if err != nil {
					s.config.Logger.Warn("terminal control decode failed", "workspace_id", workspaceId, "session_id", sessionId, "error", err)
					_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: terminalproto.ErrorCodeBadControl, Message: terminalproto.ErrorMessageBadControl})
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
				case terminalproto.TypeTakeControl:
					stream.TakeControl()
				case terminalproto.TypeDetach:
					s.config.Logger.Info("terminal browser stream detached", "workspace_id", workspaceId, "session_id", sessionId, "stream_id", streamId, "reason", "browser_detached")
					stream.Detach("browser_detached")
					return
				case terminalproto.TypePing:
					_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypePong, Nonce: message.Nonce})
				}
			case websocket.MessageBinary:
				if err := stream.WriteInput(data); err != nil {
					if terminalapp.IsNotController(err) {
						_ = writeTerminalControl(conn, &agent.ServerControlMessage{Type: terminalproto.TypeError, Code: terminalproto.ErrorCodeNotController, Message: terminalproto.ErrorMessageNotController})
						continue
					}
					s.config.Logger.Warn("terminal input write failed", "workspace_id", workspaceId, "session_id", sessionId, "bytes", len(data), "error", err)
				}
			}
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			s.config.Logger.Info("terminal browser stream context ended", "workspace_id", workspaceId, "session_id", sessionId, "stream_id", streamId, "error", r.Context().Err())
			return nil
		case <-done:
			s.config.Logger.Info("terminal browser stream input ended", "workspace_id", workspaceId, "session_id", sessionId, "stream_id", streamId)
			return nil
		case outbound, ok := <-stream.Outbound():
			if !ok {
				s.config.Logger.Info("terminal runtime outbound closed", "workspace_id", workspaceId, "session_id", sessionId, "stream_id", streamId)
				_ = conn.Close(websocket.StatusNormalClosure, "terminal closed")
				return nil
			}
			if outbound.Kind == terminalapp.OutboundBinary {
				err := conn.Write(r.Context(), websocket.MessageBinary, outbound.Binary)
				stream.MarkSent(outbound)
				if err != nil {
					s.config.Logger.Warn("terminal browser stream write failed", "workspace_id", workspaceId, "session_id", sessionId, "stream_id", streamId, "frame_kind", "binary", "error", err)
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
				s.config.Logger.Warn("terminal browser stream write failed", "workspace_id", workspaceId, "session_id", sessionId, "stream_id", streamId, "frame_kind", "control", "error", err)
				return err
			}
		}
	}
}
