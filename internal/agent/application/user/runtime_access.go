package application

import (
	"context"

	terminalapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/terminal"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
)

type RuntimeAccess interface {
	ListWorkspaces(ctx context.Context) ([]*agent.Workspace, error)
	WorkspaceTree(ctx context.Context) ([]*agent.WorkspaceTreeNode, error)
	UpdateWorkspaceOrder(ctx context.Context, workspaceIds []string) ([]*agent.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceId string) error
	ListSessionsByWorkspaceId(ctx context.Context, workspaceId string) ([]*agent.SessionSummary, error)
	UpdateSessionOrder(ctx context.Context, workspaceId string, sessionIds []string) ([]*agent.SessionSummary, error)
	CreateSession(ctx context.Context, request *agent.CreateSessionReq) (*agent.CreateSessionResp, error)
	RerunSession(ctx context.Context, workspaceId string, sessionId string, request *agent.RerunSessionReq) (*agent.CreateSessionResp, error)
	GetSession(ctx context.Context, workspaceId string, sessionId string) (*agent.SessionSummary, error)
	UpdateSession(ctx context.Context, workspaceId string, sessionId string, request *agent.UpdateSessionReq) (*agent.SessionSummary, error)
	DeleteSession(ctx context.Context, workspaceId string, sessionId string) error
	CloseSession(ctx context.Context, workspaceId string, sessionId string) (*agent.SessionSummary, error)
	ReadHistory(ctx context.Context, workspaceId string, sessionId string) ([]byte, error)
	Attach(ctx context.Context, workspaceId string, sessionId string) (TerminalStream, error)
}

type TerminalStream interface {
	Outbound() <-chan terminalapp.Outbound
	MarkSent(terminalapp.Outbound)
	WriteInput(data []byte) error
	Resize(cols int, rows int) error
	Detach(reason string)
}

type WebTerminalAccess struct {
	Registry *terminalapp.Registry
}

func (a WebTerminalAccess) ListWorkspaces(ctx context.Context) ([]*agent.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.ListWorkspaces()
}

func (a WebTerminalAccess) WorkspaceTree(ctx context.Context) ([]*agent.WorkspaceTreeNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.WorkspaceTree()
}

func (a WebTerminalAccess) UpdateWorkspaceOrder(ctx context.Context, workspaceIds []string) ([]*agent.Workspace, error) {
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

func (a WebTerminalAccess) ListSessionsByWorkspaceId(ctx context.Context, workspaceId string) ([]*agent.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.ListSessionsByWorkspaceId(workspaceId)
}

func (a WebTerminalAccess) UpdateSessionOrder(ctx context.Context, workspaceId string, sessionIds []string) ([]*agent.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.UpdateSessionOrder(workspaceId, sessionIds)
}

func (a WebTerminalAccess) CreateSession(ctx context.Context, request *agent.CreateSessionReq) (*agent.CreateSessionResp, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.CreateSession(ctx, request)
}

func (a WebTerminalAccess) RerunSession(ctx context.Context, workspaceId string, sessionId string, request *agent.RerunSessionReq) (*agent.CreateSessionResp, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.RerunSession(ctx, workspaceId, sessionId, request)
}

func (a WebTerminalAccess) GetSession(ctx context.Context, workspaceId string, sessionId string) (*agent.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.GetSession(workspaceId, sessionId)
}

func (a WebTerminalAccess) UpdateSession(ctx context.Context, workspaceId string, sessionId string, request *agent.UpdateSessionReq) (*agent.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.UpdateSession(workspaceId, sessionId, request)
}

func (a WebTerminalAccess) DeleteSession(ctx context.Context, workspaceId string, sessionId string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.Registry.DeleteSession(workspaceId, sessionId)
}

func (a WebTerminalAccess) CloseSession(ctx context.Context, workspaceId string, sessionId string) (*agent.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
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
