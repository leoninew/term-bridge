package application

import (
	"context"

	fileapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/file"
	gitapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/git"
	shortcutapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/shortcut"
	terminalapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/terminal"
	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
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
	ListShortcuts(ctx context.Context) ([]*agent.Shortcut, error)
	CreateShortcut(ctx context.Context, request *agent.CreateShortcutReq) (*agent.Shortcut, error)
	UpdateShortcut(ctx context.Context, shortcutId string, request *agent.UpdateShortcutReq) (*agent.Shortcut, error)
	UpdateShortcutOrder(ctx context.Context, shortcutIds []string) ([]*agent.Shortcut, error)
	DeleteShortcut(ctx context.Context, shortcutId string) error
	FsStat(ctx context.Context, request *agent.FsStatReq) (*agent.FsStatResp, error)
	FsReadDirectory(ctx context.Context, request *agent.FsReadDirectoryReq) (*agent.FsReadDirectoryResp, error)
	FsReadFile(ctx context.Context, request *agent.FsReadFileReq) (*agent.FsReadFileResp, error)
	FsWriteFile(ctx context.Context, request *agent.FsWriteFileReq) (*agent.FsWriteFileResp, error)
	FsCreateDirectory(ctx context.Context, request *agent.FsCreateDirectoryReq) (*agent.FsCreateDirectoryResp, error)
	FsDelete(ctx context.Context, request *agent.FsDeleteReq) (*agent.FsDeleteResp, error)
	FsRename(ctx context.Context, request *agent.FsRenameReq) (*agent.FsRenameResp, error)
	SubscribeWorkspaceChanges(ctx context.Context, workspaceId string) (filemodel.WorkspaceChangeSubscription, error)
	ScmStatus(ctx context.Context, request *agent.ScmStatusReq) (*agent.ScmStatusResp, error)
	ScmOriginalContent(ctx context.Context, request *agent.ScmOriginalContentReq) (*agent.ScmOriginalContentResp, error)
	ScmExecute(ctx context.Context, request *agent.ScmExecuteReq) (*agent.ScmExecuteResp, error)
	ScmRepository(ctx context.Context, request *agent.ScmRepositoryReq) (*agent.ScmRepositoryResp, error)
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
	TakeControl()
	IsController() bool
	Detach(reason string)
}

type WebTerminalAccess struct {
	Registry  *terminalapp.Registry
	Shortcuts shortcutapp.Service
	Files     *fileapp.Service
	Git       *gitapp.Service
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

func (a WebTerminalAccess) ListShortcuts(ctx context.Context) ([]*agent.Shortcut, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Shortcuts.List()
}

func (a WebTerminalAccess) CreateShortcut(ctx context.Context, request *agent.CreateShortcutReq) (*agent.Shortcut, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Shortcuts.Create(request)
}

func (a WebTerminalAccess) UpdateShortcut(ctx context.Context, shortcutId string, request *agent.UpdateShortcutReq) (*agent.Shortcut, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Shortcuts.Update(shortcutId, request)
}

func (a WebTerminalAccess) UpdateShortcutOrder(ctx context.Context, shortcutIds []string) ([]*agent.Shortcut, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.Shortcuts.UpdateOrder(&agent.UpdateShortcutOrderReq{ShortcutIds: shortcutIds})
}

func (a WebTerminalAccess) DeleteShortcut(ctx context.Context, shortcutId string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.Shortcuts.Delete(shortcutId)
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
