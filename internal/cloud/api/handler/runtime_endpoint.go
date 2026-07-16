package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

type runtimeEndpoint interface {
	JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error)
	History(ctx context.Context, workspaceId string, sessionId string, requestId string) (text string, offline bool, err error)
	Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error
	Available() bool
}

type tunnelRuntimeEndpoint struct {
	route    *agentRoute
	deviceId string
	handler  *Handler
}

func (e tunnelRuntimeEndpoint) Available() bool {
	return e.route != nil
}

func (e tunnelRuntimeEndpoint) JSON(ctx context.Context, method string, params any, requestId string) (json.RawMessage, error) {
	request, err := runtimeRequestFrame(method, params, requestId)
	if err != nil {
		return nil, err
	}
	response, err := e.route.request(ctx, request)
	if err != nil {
		return nil, err
	}
	return tunnel.ResponseJSON(response)
}

func (e tunnelRuntimeEndpoint) History(ctx context.Context, workspaceId string, sessionId string, requestId string) (string, bool, error) {
	request := &shared.TunnelFrame{RequestId: requestId, Payload: &shared.TunnelFrame_ReadHistoryReq{ReadHistoryReq: &agent.ReadWorkspaceSessionHistoryReq{WorkspaceId: workspaceId, SessionId: sessionId}}}
	response, err := e.route.request(ctx, request)
	if err != nil {
		return "", false, err
	}
	if response.GetReadHistoryResp() == nil {
		if response.GetError() != nil {
			return "", false, errors.New(tunnel.ErrorMessage(response))
		}
		return "", false, fmt.Errorf("unexpected history response payload %T", response.GetPayload())
	}
	return response.GetReadHistoryResp().GetText(), false, nil
}

func (e tunnelRuntimeEndpoint) Attach(w http.ResponseWriter, r *http.Request, workspaceId string, sessionId string) error {
	e.handler.handleTerminalWS(w, r, e.route, workspaceId, sessionId)
	return nil
}

func runtimeRequestFrame(method string, params any, requestId string) (*shared.TunnelFrame, error) {
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
	case "list_files":
		req, ok := params.(*agent.ListFilesReq)
		if !ok {
			return nil, fmt.Errorf("list_files params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_ListFilesReq{ListFilesReq: req}
	case "read_file":
		req, ok := params.(*agent.ReadFileReq)
		if !ok {
			return nil, fmt.Errorf("read_file params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_ReadFileReq{ReadFileReq: req}
	case "create_file":
		req, ok := params.(*agent.CreateFileReq)
		if !ok {
			return nil, fmt.Errorf("create_file params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_CreateFileReq{CreateFileReq: req}
	case "create_directory":
		req, ok := params.(*agent.CreateDirectoryReq)
		if !ok {
			return nil, fmt.Errorf("create_directory params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_CreateDirectoryReq{CreateDirectoryReq: req}
	case "write_file":
		req, ok := params.(*agent.WriteFileReq)
		if !ok {
			return nil, fmt.Errorf("write_file params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_WriteFileReq{WriteFileReq: req}
	case "rename_entry":
		req, ok := params.(*agent.RenameEntryReq)
		if !ok {
			return nil, fmt.Errorf("rename_entry params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_RenameEntryReq{RenameEntryReq: req}
	case "move_entry":
		req, ok := params.(*agent.MoveEntryReq)
		if !ok {
			return nil, fmt.Errorf("move_entry params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_MoveEntryReq{MoveEntryReq: req}
	case "delete_entry":
		req, ok := params.(*agent.DeleteEntryReq)
		if !ok {
			return nil, fmt.Errorf("delete_entry params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_DeleteEntryReq{DeleteEntryReq: req}
	case "git_status":
		req, ok := params.(*agent.GitStatusReq)
		if !ok {
			return nil, fmt.Errorf("git_status params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_GitStatusReq{GitStatusReq: req}
	case "git_diff":
		req, ok := params.(*agent.GitDiffReq)
		if !ok {
			return nil, fmt.Errorf("git_diff params have type %T", params)
		}
		frame.Payload = &shared.TunnelFrame_GitDiffReq{GitDiffReq: req}
	default:
		return nil, fmt.Errorf("unknown runtime method %q", method)
	}
	return frame, nil
}
