package file

import (
	"context"

	workspace "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
)

type WorkspaceLocator interface {
	FindWorkspaceById(workspaceId string) (workspace.Workspace, error)
}

type Store interface {
	List(ctx context.Context, root string, directory RelativePath) (ListResult, error)
	Read(ctx context.Context, root string, path RelativePath) (ReadResult, error)
	CreateFile(ctx context.Context, root string, request CreateFileRequest) (MutationResult, error)
	CreateDirectory(ctx context.Context, root string, request CreateDirectoryRequest) (MutationResult, error)
	Write(ctx context.Context, root string, request WriteRequest) (MutationResult, error)
	Rename(ctx context.Context, root string, request RenameRequest) (MutationResult, error)
	Move(ctx context.Context, root string, request MoveRequest) (MutationResult, error)
	Delete(ctx context.Context, root string, request DeleteRequest) (MutationResult, error)
}
