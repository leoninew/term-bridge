package agent

import (
	"context"

	terminalapp "termbridge-go/internal/application/terminal"
)

type RuntimeAccess interface {
	WorkspaceTree(ctx context.Context) ([]terminalapp.WorkspaceTreeNode, error)
	ListSessions(ctx context.Context) ([]terminalapp.SessionSummary, error)
	ReadHistory(ctx context.Context, sessionID string) ([]byte, error)
	Attach(ctx context.Context, sessionID string) (TerminalStream, error)
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

func (a WebTerminalAccess) WorkspaceTree(ctx context.Context) ([]terminalapp.WorkspaceTreeNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.WorkspaceTree()
}

func (a WebTerminalAccess) ListSessions(ctx context.Context) ([]terminalapp.SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.ListSessions()
}

func (a WebTerminalAccess) ReadHistory(ctx context.Context, sessionID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.History(sessionID)
}

func (a WebTerminalAccess) Attach(ctx context.Context, sessionID string) (TerminalStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.Attach(sessionID)
}
