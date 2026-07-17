package file

import (
	"context"
	"errors"
	"os"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
)

type Service struct {
	config     Config
	workspaces filemodel.WorkspaceLocator
	store      filemodel.Store
	changes    filemodel.WorkspaceChangeSource
}

func NewService(config Config, workspaces filemodel.WorkspaceLocator, store filemodel.Store, changes filemodel.WorkspaceChangeSource) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if workspaces == nil {
		return nil, errors.New("workspace locator is required")
	}
	if store == nil {
		return nil, errors.New("workspace file store is required")
	}
	if changes == nil {
		return nil, errors.New("workspace change source is required")
	}
	return &Service{config: config, workspaces: workspaces, store: store, changes: changes}, nil
}

func (s *Service) Subscribe(ctx context.Context, workspaceId string) (filemodel.WorkspaceChangeSubscription, error) {
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return nil, err
	}
	operation.Cancel()
	return s.changes.Subscribe(ctx, operation.Root)
}

func (s *Service) List(ctx context.Context, workspaceId string, path string) (filemodel.ListResult, error) {
	parsed, err := filemodel.ParseRelativePath(path, true)
	if err != nil {
		return filemodel.ListResult{}, err
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return filemodel.ListResult{}, err
	}
	defer operation.Cancel()
	return s.store.List(operation.Context, operation.Root, parsed)
}

func (s *Service) Stat(ctx context.Context, workspaceId string, path string) (filemodel.Entry, error) {
	// Root path "" is valid (workspace root directory).
	parsed, err := filemodel.ParseRelativePath(path, true)
	if err != nil {
		return filemodel.Entry{}, err
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return filemodel.Entry{}, err
	}
	defer operation.Cancel()
	return s.store.Stat(operation.Context, operation.Root, parsed)
}

func (s *Service) Read(ctx context.Context, workspaceId string, path string) (filemodel.ReadResult, error) {
	parsed, err := filemodel.ParseRelativePath(path, false)
	if err != nil {
		return filemodel.ReadResult{}, err
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return filemodel.ReadResult{}, err
	}
	defer operation.Cancel()
	return s.store.Read(operation.Context, operation.Root, parsed)
}

func (s *Service) CreateFile(ctx context.Context, workspaceId string, request filemodel.CreateFileRequest) (filemodel.MutationResult, error) {
	if request.Path == "" {
		return filemodel.MutationResult{}, filemodel.InvalidPath("path is required")
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return filemodel.MutationResult{}, err
	}
	defer operation.Cancel()
	return s.store.CreateFile(operation.Context, operation.Root, request)
}

func (s *Service) CreateDirectory(ctx context.Context, workspaceId string, request filemodel.CreateDirectoryRequest) (filemodel.MutationResult, error) {
	if request.Path == "" {
		return filemodel.MutationResult{}, filemodel.InvalidPath("path is required")
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return filemodel.MutationResult{}, err
	}
	defer operation.Cancel()
	return s.store.CreateDirectory(operation.Context, operation.Root, request)
}

func (s *Service) Write(ctx context.Context, workspaceId string, request filemodel.WriteRequest) (filemodel.MutationResult, error) {
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return filemodel.MutationResult{}, err
	}
	defer operation.Cancel()
	return s.store.Write(operation.Context, operation.Root, request)
}

func (s *Service) Rename(ctx context.Context, workspaceId string, request filemodel.RenameRequest) (filemodel.MutationResult, error) {
	if _, err := filemodel.ParseName(request.NewName); err != nil {
		return filemodel.MutationResult{}, err
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return filemodel.MutationResult{}, err
	}
	defer operation.Cancel()
	return s.store.Rename(operation.Context, operation.Root, request)
}

func (s *Service) Move(ctx context.Context, workspaceId string, request filemodel.MoveRequest) (filemodel.MutationResult, error) {
	if request.SourcePath == "" || request.DestinationPath == "" {
		return filemodel.MutationResult{}, filemodel.InvalidPath("source and destination paths are required")
	}
	if request.DestinationPath.IsWithin(request.SourcePath) && request.SourcePath != request.DestinationPath {
		return filemodel.MutationResult{}, filemodel.NewError("invalid_operation", "A directory cannot be moved into itself.")
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return filemodel.MutationResult{}, err
	}
	defer operation.Cancel()
	return s.store.Move(operation.Context, operation.Root, request)
}

func (s *Service) Delete(ctx context.Context, workspaceId string, request filemodel.DeleteRequest) (filemodel.MutationResult, error) {
	if request.Path == "" {
		return filemodel.MutationResult{}, filemodel.NewError("root_protected", "The workspace root cannot be deleted.")
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return filemodel.MutationResult{}, err
	}
	defer operation.Cancel()
	return s.store.Delete(operation.Context, operation.Root, request)
}

type rootOperation struct {
	Root    string
	Context context.Context
	Cancel  context.CancelFunc
}

func (s *Service) root(ctx context.Context, workspaceId string) (rootOperation, error) {
	if err := ctx.Err(); err != nil {
		return rootOperation{}, err
	}
	workspace, err := s.workspaces.FindWorkspaceById(workspaceId)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return rootOperation{}, filemodel.NewError("workspace_not_found", "Workspace was not found.")
		}
		return rootOperation{}, err
	}
	operationCtx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	return rootOperation{Root: workspace.Path, Context: operationCtx, Cancel: cancel}, nil
}
