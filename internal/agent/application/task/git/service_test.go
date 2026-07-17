package git

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
	workspace "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
)

type serviceTestWorkspaceLocator struct {
	find      func(string) (workspace.Workspace, error)
	workspace workspace.Workspace
	err       error
}

func (l serviceTestWorkspaceLocator) FindWorkspaceById(workspaceId string) (workspace.Workspace, error) {
	if l.find != nil {
		return l.find(workspaceId)
	}
	return l.workspace, l.err
}

type serviceTestRepository struct {
	identity func(context.Context, string) (gitmodel.RepositoryIdentity, error)
	status   func(context.Context, string) (gitmodel.StatusResult, error)
	diff     func(context.Context, string, gitmodel.DiffRequest) (gitmodel.DiffResult, error)
	mutate   func(context.Context, string, gitmodel.PathMutationRequest) (gitmodel.OperationResult, error)
}

func (r serviceTestRepository) Identity(ctx context.Context, root string) (gitmodel.RepositoryIdentity, error) {
	if r.identity == nil {
		return gitmodel.RepositoryIdentity{State: gitmodel.StateAvailable, Key: "repository-root"}, nil
	}
	return r.identity(ctx, root)
}

func (r serviceTestRepository) Status(ctx context.Context, root string) (gitmodel.StatusResult, error) {
	if r.status == nil {
		return gitmodel.StatusResult{}, nil
	}
	return r.status(ctx, root)
}

func (r serviceTestRepository) Diff(ctx context.Context, root string, request gitmodel.DiffRequest) (gitmodel.DiffResult, error) {
	if r.diff == nil {
		return gitmodel.DiffResult{}, nil
	}
	return r.diff(ctx, root, request)
}

func (serviceTestRepository) Summary(context.Context, string) (gitmodel.RepositorySummary, error) {
	return gitmodel.RepositorySummary{}, nil
}

func (r serviceTestRepository) MutatePath(ctx context.Context, root string, request gitmodel.PathMutationRequest) (gitmodel.OperationResult, error) {
	if r.mutate == nil {
		return gitmodel.OperationResult{}, nil
	}
	return r.mutate(ctx, root, request)
}

func (serviceTestRepository) Commit(context.Context, string, gitmodel.CommitRequest) (gitmodel.OperationResult, error) {
	return gitmodel.OperationResult{}, nil
}

func (serviceTestRepository) CreateBranch(context.Context, string, gitmodel.BranchRequest) (gitmodel.OperationResult, error) {
	return gitmodel.OperationResult{}, nil
}

func (serviceTestRepository) SwitchBranch(context.Context, string, gitmodel.BranchRequest) (gitmodel.OperationResult, error) {
	return gitmodel.OperationResult{}, nil
}

func TestDiffRejectsInvalidLayerBeforeWorkspaceLookup(t *testing.T) {
	service, err := NewService(
		Config{Executable: "git", CommandTimeout: time.Second, MaxStdoutBytes: 1024, MaxStderrBytes: 1024, MaxTextBytes: 1024},
		serviceTestWorkspaceLocator{err: errors.New("workspace lookup must not run")},
		serviceTestRepository{},
	)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	_, err = service.Diff(context.Background(), "workspace-1", gitmodel.DiffRequest{Path: "notes.txt", Layer: "invalid"})
	if err == nil {
		t.Fatal("Diff() error = nil, want invalid Git layer error")
	}
	if err.Error() != "The Git diff layer is invalid." {
		t.Fatalf("Diff() error = %v, want invalid Git layer error", err)
	}
}

func TestStatusMapsMissingWorkspaceBeforeRepositoryAccess(t *testing.T) {
	repositoryCalled := false
	service, err := NewService(
		Config{Executable: "git", CommandTimeout: time.Second, MaxStdoutBytes: 1024, MaxStderrBytes: 1024, MaxTextBytes: 1024},
		serviceTestWorkspaceLocator{err: os.ErrNotExist},
		serviceTestRepository{status: func(context.Context, string) (gitmodel.StatusResult, error) {
			repositoryCalled = true
			return gitmodel.StatusResult{}, nil
		}},
	)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	_, err = service.Status(context.Background(), "missing-workspace")
	if err == nil {
		t.Fatal("Status() error = nil, want missing workspace error")
	}
	if err.Error() != "Workspace was not found." {
		t.Fatalf("Status() error = %v, want missing workspace error", err)
	}
	if repositoryCalled {
		t.Fatal("repository was called for a missing workspace")
	}
}

func TestMutatePathRejectsInvalidRequestBeforeWorkspaceLookup(t *testing.T) {
	service, err := NewService(
		Config{Executable: "git", CommandTimeout: time.Second, MaxStdoutBytes: 1024, MaxStderrBytes: 1024, MaxTextBytes: 1024},
		serviceTestWorkspaceLocator{err: errors.New("workspace lookup must not run")},
		serviceTestRepository{},
	)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	result, err := service.MutatePath(context.Background(), "workspace-1", gitmodel.PathMutationRequest{
		Path: "notes.txt", Layer: gitmodel.LayerStaged, Mutation: gitmodel.MutationStage,
	})
	if err != nil {
		t.Fatalf("MutatePath() error = %v", err)
	}
	if result.State != gitmodel.OperationStateConflict {
		t.Fatalf("mutation state = %q, want %q", result.State, gitmodel.OperationStateConflict)
	}
}

func TestStatusAndMutationShareRepositoryLockAcrossNestedWorkspaces(t *testing.T) {
	statusStarted := make(chan struct{})
	statusRelease := make(chan struct{})
	mutationStarted := make(chan struct{})
	service, err := NewService(
		Config{Executable: "git", CommandTimeout: time.Second, MaxStdoutBytes: 1024, MaxStderrBytes: 1024, MaxTextBytes: 1024},
		serviceTestWorkspaceLocator{find: func(workspaceId string) (workspace.Workspace, error) {
			if workspaceId == "workspace-1" {
				return workspace.Workspace{Path: "workspace-one"}, nil
			}
			return workspace.Workspace{Path: "workspace-two"}, nil
		}},
		serviceTestRepository{
			identity: func(context.Context, string) (gitmodel.RepositoryIdentity, error) {
				return gitmodel.RepositoryIdentity{State: gitmodel.StateAvailable, Key: "canonical-repository-root"}, nil
			},
			status: func(context.Context, string) (gitmodel.StatusResult, error) {
				close(statusStarted)
				<-statusRelease
				return gitmodel.StatusResult{State: gitmodel.StateAvailable}, nil
			},
			mutate: func(context.Context, string, gitmodel.PathMutationRequest) (gitmodel.OperationResult, error) {
				close(mutationStarted)
				return gitmodel.OperationResult{State: gitmodel.OperationStateAvailable}, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	statusDone := make(chan struct{})
	go func() {
		defer close(statusDone)
		if _, err := service.Status(context.Background(), "workspace-1"); err != nil {
			t.Errorf("Status() error = %v", err)
		}
	}()
	<-statusStarted
	mutationDone := make(chan struct{})
	go func() {
		defer close(mutationDone)
		if _, err := service.MutatePath(context.Background(), "workspace-2", gitmodel.PathMutationRequest{
			Path: "notes.txt", Layer: gitmodel.LayerUnstaged, Mutation: gitmodel.MutationStage,
		}); err != nil {
			t.Errorf("MutatePath() error = %v", err)
		}
	}()
	select {
	case <-mutationStarted:
		t.Fatal("mutation began while a repository read held the shared lock")
	case <-time.After(25 * time.Millisecond):
	}
	close(statusRelease)
	<-statusDone
	select {
	case <-mutationStarted:
	case <-time.After(time.Second):
		t.Fatal("mutation did not begin after the repository read completed")
	}
	<-mutationDone
}

func TestStatusPropagatesCommandDeadlineToRepository(t *testing.T) {
	service, err := NewService(
		Config{Executable: "git", CommandTimeout: time.Millisecond, MaxStdoutBytes: 1024, MaxStderrBytes: 1024, MaxTextBytes: 1024},
		serviceTestWorkspaceLocator{workspace: workspace.Workspace{Path: "workspace-root"}},
		serviceTestRepository{status: func(ctx context.Context, root string) (gitmodel.StatusResult, error) {
			if root != "workspace-root" {
				t.Fatalf("repository root = %q, want workspace-root", root)
			}
			<-ctx.Done()
			return gitmodel.StatusResult{State: gitmodel.StateUnavailable}, nil
		}},
	)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	result, err := service.Status(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if result.State != gitmodel.StateUnavailable {
		t.Fatalf("status state = %q, want %q", result.State, gitmodel.StateUnavailable)
	}
}
