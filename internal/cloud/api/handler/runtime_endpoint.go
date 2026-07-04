package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	runtimev1 "termbridge-go/internal/shared/dto/proto/termbridge/runtime/v1"
	tunnelv1 "termbridge-go/internal/shared/dto/proto/termbridge/tunnel/v1"
	"termbridge-go/internal/shared/dto/protocol/tunnel"
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
	request := &tunnelv1.TunnelFrame{RequestId: requestId, Payload: &tunnelv1.TunnelFrame_ReadHistoryReq{ReadHistoryReq: &runtimev1.ReadHistoryReq{WorkspaceId: workspaceId, SessionId: sessionId}}}
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

func runtimeRequestFrame(method string, params any, requestId string) (*tunnelv1.TunnelFrame, error) {
	frame := &tunnelv1.TunnelFrame{RequestId: requestId}
	switch method {
	case "workspaces":
		frame.Payload = &tunnelv1.TunnelFrame_ListWorkspacesReq{ListWorkspacesReq: &runtimev1.ListWorkspacesReq{}}
	case "workspace_tree":
		frame.Payload = &tunnelv1.TunnelFrame_WorkspaceTreeReq{WorkspaceTreeReq: &runtimev1.WorkspaceTreeReq{}}
	case "workspace_order":
		req, ok := params.(UpdateWorkspaceOrderReq)
		if !ok {
			return nil, fmt.Errorf("workspace_order params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_UpdateWorkspaceOrderReq{UpdateWorkspaceOrderReq: &runtimev1.UpdateWorkspaceOrderReq{WorkspaceIds: req.WorkspaceIds}}
	case "delete_workspace":
		req, ok := params.(DeleteWorkspaceReq)
		if !ok {
			return nil, fmt.Errorf("delete_workspace params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_DeleteWorkspaceReq{DeleteWorkspaceReq: &runtimev1.DeleteWorkspaceReq{WorkspaceId: req.WorkspaceId}}
	case "workspace_sessions":
		req, ok := params.(WorkspaceSessionsReq)
		if !ok {
			return nil, fmt.Errorf("workspace_sessions params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_WorkspaceSessionsReq{WorkspaceSessionsReq: &runtimev1.WorkspaceSessionsReq{WorkspaceId: req.WorkspaceId}}
	case "session_order":
		req, ok := params.(WorkspaceSessionOrderReq)
		if !ok {
			return nil, fmt.Errorf("session_order params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_UpdateSessionOrderReq{UpdateSessionOrderReq: &runtimev1.UpdateSessionOrderReq{WorkspaceId: req.WorkspaceId, SessionIds: req.SessionIds}}
	case "create_session":
		req, ok := params.(CreateSessionReq)
		if !ok {
			return nil, fmt.Errorf("create_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_CreateSessionReq{CreateSessionReq: &runtimev1.CreateSessionReq{WorkspaceId: req.WorkspaceId, Name: req.Name, Cwd: req.Cwd, Command: req.Command, Cols: int32(req.Cols), Rows: int32(req.Rows)}}
	case "rerun_session":
		req, ok := params.(RerunWorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("rerun_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_RerunSessionReq{RerunSessionReq: &runtimev1.RerunSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId, Cols: int32(req.Request.Cols), Rows: int32(req.Request.Rows)}}
	case "get_session":
		req, ok := params.(WorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("get_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_GetSessionReq{GetSessionReq: &runtimev1.WorkspaceSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId}}
	case "update_session":
		req, ok := params.(UpdateWorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("update_session params have type %T", params)
		}
		name := req.Request.Name
		frame.Payload = &tunnelv1.TunnelFrame_UpdateSessionReq{UpdateSessionReq: &runtimev1.UpdateSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId, Name: &name}}
	case "delete_session":
		req, ok := params.(WorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("delete_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_DeleteSessionReq{DeleteSessionReq: &runtimev1.WorkspaceSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId}}
	case "close_session":
		req, ok := params.(WorkspaceSessionReq)
		if !ok {
			return nil, fmt.Errorf("close_session params have type %T", params)
		}
		frame.Payload = &tunnelv1.TunnelFrame_CloseSessionReq{CloseSessionReq: &runtimev1.WorkspaceSessionReq{WorkspaceId: req.WorkspaceId, SessionId: req.SessionId}}
	default:
		return nil, fmt.Errorf("unknown runtime method %q", method)
	}
	return frame, nil
}
