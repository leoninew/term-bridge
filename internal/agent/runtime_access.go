package agent

import (
	"context"

	"termbridge-go/internal/webterminal"
)

type RuntimeAccess interface {
	WorkspaceTree(ctx context.Context) ([]webterminal.WorkspaceTreeNode, error)
	ListSessions(ctx context.Context) ([]webterminal.SessionSummary, error)
	ReadHistory(ctx context.Context, sessionID string) ([]byte, error)
	Attach(ctx context.Context, sessionID string) (TerminalStream, error)
}

type TerminalStream interface {
	Outbound() <-chan webterminal.Outbound
	WriteInput(data []byte) error
	Resize(cols int, rows int) error
	Detach(reason string)
}

type WebTerminalAccess struct {
	Registry *webterminal.Registry
}

func (a WebTerminalAccess) WorkspaceTree(ctx context.Context) ([]webterminal.WorkspaceTreeNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Registry.WorkspaceTree()
}

func (a WebTerminalAccess) ListSessions(ctx context.Context) ([]webterminal.SessionSummary, error) {
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
