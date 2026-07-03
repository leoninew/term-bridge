package application

import (
	"context"

	terminalapp "termbridge-go/internal/agent/application/terminal"
)

type RuntimeAccess interface {
	ListWorkspaces(ctx context.Context) ([]terminalapp.WorkspaceSummary, error)
	WorkspaceTree(ctx context.Context) ([]terminalapp.WorkspaceTreeNode, error)
	UpdateWorkspaceOrder(ctx context.Context, workspaceIds []string) ([]terminalapp.WorkspaceSummary, error)
	DeleteWorkspace(ctx context.Context, workspaceId string) error
	ListSessionsByWorkspaceId(ctx context.Context, workspaceId string) ([]terminalapp.WorkspaceSessionSummary, error)
	UpdateSessionOrder(ctx context.Context, workspaceId string, sessionIds []string) ([]terminalapp.WorkspaceSessionSummary, error)
	CreateSession(ctx context.Context, request terminalapp.CreateSessionReq) (terminalapp.CreateSessionResp, error)
	RerunSession(ctx context.Context, workspaceId string, sessionId string, request terminalapp.RerunSessionReq) (terminalapp.CreateSessionResp, error)
	GetSession(ctx context.Context, workspaceId string, sessionId string) (terminalapp.SessionSummary, error)
	UpdateSession(ctx context.Context, workspaceId string, sessionId string, request terminalapp.UpdateSessionReq) (terminalapp.SessionSummary, error)
	DeleteSession(ctx context.Context, workspaceId string, sessionId string) error
	CloseSession(ctx context.Context, workspaceId string, sessionId string) (terminalapp.SessionSummary, error)
	ReadHistory(ctx context.Context, workspaceId string, sessionId string) ([]byte, error)
	Attach(ctx context.Context, workspaceId string, sessionId string) (TerminalStream, error)
}

type TerminalStream interface {
	Outbound() <-chan terminalapp.Outbound
	WriteInput(data []byte) error
	Resize(cols int, rows int) error
	Detach(reason string)
}

type WebTerminalAccess struct {
	Registry *terminalapp.Registry
}

func (a WebTerminalAccess) ListWorkspaces(ctx context.Context) ([]terminalapp.WorkspaceSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.ListWorkspaces()
}

func (a WebTerminalAccess) WorkspaceTree(ctx context.Context) ([]terminalapp.WorkspaceTreeNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.WorkspaceTree()
}

func (a WebTerminalAccess) UpdateWorkspaceOrder(ctx context.Context, workspaceIds []string) ([]terminalapp.WorkspaceSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.UpdateWorkspaceOrder(workspaceIds)
}

func (a WebTerminalAccess) DeleteWorkspace(ctx context.Context, workspaceId string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.Registry.DeleteWorkspace(workspaceId)
}

func (a WebTerminalAccess) ListSessionsByWorkspaceId(ctx context.Context, workspaceId string) ([]terminalapp.WorkspaceSessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.ListSessionsByWorkspaceId(workspaceId)
}

func (a WebTerminalAccess) UpdateSessionOrder(ctx context.Context, workspaceId string, sessionIds []string) ([]terminalapp.WorkspaceSessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.UpdateSessionOrder(workspaceId, sessionIds)
}

func (a WebTerminalAccess) CreateSession(ctx context.Context, request terminalapp.CreateSessionReq) (terminalapp.CreateSessionResp, error) {
	if err := ctx.Err(); err != nil {
		return terminalapp.CreateSessionResp{}, err
	}
	return a.Registry.CreateSession(ctx, request)
}

func (a WebTerminalAccess) RerunSession(ctx context.Context, workspaceId string, sessionId string, request terminalapp.RerunSessionReq) (terminalapp.CreateSessionResp, error) {
	if err := ctx.Err(); err != nil {
		return terminalapp.CreateSessionResp{}, err
	}
	return a.Registry.RerunSession(ctx, workspaceId, sessionId, request)
}

func (a WebTerminalAccess) GetSession(ctx context.Context, workspaceId string, sessionId string) (terminalapp.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return terminalapp.SessionSummary{}, err
	}
	return a.Registry.GetSession(workspaceId, sessionId)
}

func (a WebTerminalAccess) UpdateSession(ctx context.Context, workspaceId string, sessionId string, request terminalapp.UpdateSessionReq) (terminalapp.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return terminalapp.SessionSummary{}, err
	}
	return a.Registry.UpdateSession(workspaceId, sessionId, request)
}

func (a WebTerminalAccess) DeleteSession(ctx context.Context, workspaceId string, sessionId string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.Registry.DeleteSession(workspaceId, sessionId)
}

func (a WebTerminalAccess) CloseSession(ctx context.Context, workspaceId string, sessionId string) (terminalapp.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return terminalapp.SessionSummary{}, err
	}
	return a.Registry.CloseSession(workspaceId, sessionId, "api_close")
}

func (a WebTerminalAccess) ReadHistory(ctx context.Context, workspaceId string, sessionId string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.History(workspaceId, sessionId)
}

func (a WebTerminalAccess) Attach(ctx context.Context, workspaceId string, sessionId string) (TerminalStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.Attach(workspaceId, sessionId)
}
