package agent

import (
	"context"

	terminalapp "termbridge-go/internal/application/terminal"
)

type RuntimeAccess interface {
	ListWorkspaces(ctx context.Context) ([]terminalapp.WorkspaceSummary, error)
	WorkspaceTree(ctx context.Context) ([]terminalapp.WorkspaceTreeNode, error)
	UpdateWorkspaceOrder(ctx context.Context, workspaceIds []string) ([]terminalapp.WorkspaceSummary, error)
	DeleteWorkspace(ctx context.Context, workspaceId string) error
	ListSessionsByWorkspaceId(ctx context.Context, workspaceId string) ([]terminalapp.WorkspaceSessionSummary, error)
	ListSessions(ctx context.Context) ([]terminalapp.SessionSummary, error)
	CreateSession(ctx context.Context, request terminalapp.CreateSessionRequest) (terminalapp.CreateSessionResponse, error)
	GetSession(ctx context.Context, sessionId string) (terminalapp.SessionSummary, error)
	UpdateSession(ctx context.Context, sessionId string, request terminalapp.UpdateSessionRequest) (terminalapp.SessionSummary, error)
	DeleteSession(ctx context.Context, sessionId string) error
	CloseSession(ctx context.Context, sessionId string) (terminalapp.SessionSummary, error)
	ReadHistory(ctx context.Context, sessionId string) ([]byte, error)
	Attach(ctx context.Context, sessionId string) (TerminalStream, error)
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

func (a WebTerminalAccess) ListSessions(ctx context.Context) ([]terminalapp.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.ListSessions()
}

func (a WebTerminalAccess) CreateSession(ctx context.Context, request terminalapp.CreateSessionRequest) (terminalapp.CreateSessionResponse, error) {
	if err := ctx.Err(); err != nil {
		return terminalapp.CreateSessionResponse{}, err
	}
	return a.Registry.CreateSession(ctx, request)
}

func (a WebTerminalAccess) GetSession(ctx context.Context, sessionId string) (terminalapp.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return terminalapp.SessionSummary{}, err
	}
	return a.Registry.GetSession(sessionId)
}

func (a WebTerminalAccess) UpdateSession(ctx context.Context, sessionId string, request terminalapp.UpdateSessionRequest) (terminalapp.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return terminalapp.SessionSummary{}, err
	}
	return a.Registry.UpdateSession(sessionId, request)
}

func (a WebTerminalAccess) DeleteSession(ctx context.Context, sessionId string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.Registry.DeleteSession(sessionId)
}

func (a WebTerminalAccess) CloseSession(ctx context.Context, sessionId string) (terminalapp.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return terminalapp.SessionSummary{}, err
	}
	return a.Registry.CloseSession(sessionId, "api_close")
}

func (a WebTerminalAccess) ReadHistory(ctx context.Context, sessionId string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.History(sessionId)
}

func (a WebTerminalAccess) Attach(ctx context.Context, sessionId string) (TerminalStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.Attach(sessionId)
}
