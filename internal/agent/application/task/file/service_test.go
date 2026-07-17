package file

import (
	"context"
	"errors"
	"testing"
	"time"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	workspace "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
)

func TestServiceSubscribesToWorkspaceRoot(t *testing.T) {
	changes := &fakeChangeSource{}
	service, err := NewService(testConfig(), fakeWorkspaceLocator{workspace: workspace.Workspace{Id: "workspace-1", Path: t.TempDir()}}, fakeStore{}, changes)
	if err != nil {
		t.Fatalf("new file service: %v", err)
	}

	subscription, err := service.Subscribe(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if changes.root != serviceRoot(t, service, "workspace-1") {
		t.Fatalf("watch root = %q, want workspace root", changes.root)
	}
	if subscription != changes.subscription {
		t.Fatalf("subscription = %T, want source subscription", subscription)
	}
}

func TestServiceSubscriptionReportsWorkspaceNotFound(t *testing.T) {
	service, err := NewService(testConfig(), fakeWorkspaceLocator{err: errors.New("missing workspace")}, fakeStore{}, &fakeChangeSource{})
	if err != nil {
		t.Fatalf("new file service: %v", err)
	}

	_, err = service.Subscribe(context.Background(), "workspace-1")
	if err == nil {
		t.Fatal("subscribe error = nil, want workspace lookup error")
	}
}

func TestServiceRequiresWorkspaceChangeSource(t *testing.T) {
	_, err := NewService(testConfig(), fakeWorkspaceLocator{}, fakeStore{}, nil)
	if err == nil {
		t.Fatal("new file service error = nil, want missing change source")
	}
}

type fakeWorkspaceLocator struct {
	workspace workspace.Workspace
	err       error
}

func (f fakeWorkspaceLocator) FindWorkspaceById(string) (workspace.Workspace, error) {
	if f.err != nil {
		return workspace.Workspace{}, f.err
	}
	return f.workspace, nil
}

type fakeStore struct{}

func (fakeStore) List(context.Context, string, filemodel.RelativePath) (filemodel.ListResult, error) {
	return filemodel.ListResult{}, nil
}

func (fakeStore) Stat(context.Context, string, filemodel.RelativePath) (filemodel.Entry, error) {
	return filemodel.Entry{}, nil
}

func (fakeStore) Read(context.Context, string, filemodel.RelativePath) (filemodel.ReadResult, error) {
	return filemodel.ReadResult{}, nil
}

func (fakeStore) CreateFile(context.Context, string, filemodel.CreateFileRequest) (filemodel.MutationResult, error) {
	return filemodel.MutationResult{}, nil
}

func (fakeStore) CreateDirectory(context.Context, string, filemodel.CreateDirectoryRequest) (filemodel.MutationResult, error) {
	return filemodel.MutationResult{}, nil
}

func (fakeStore) Write(context.Context, string, filemodel.WriteRequest) (filemodel.MutationResult, error) {
	return filemodel.MutationResult{}, nil
}

func (fakeStore) Rename(context.Context, string, filemodel.RenameRequest) (filemodel.MutationResult, error) {
	return filemodel.MutationResult{}, nil
}

func (fakeStore) Move(context.Context, string, filemodel.MoveRequest) (filemodel.MutationResult, error) {
	return filemodel.MutationResult{}, nil
}

func (fakeStore) Delete(context.Context, string, filemodel.DeleteRequest) (filemodel.MutationResult, error) {
	return filemodel.MutationResult{}, nil
}

type fakeChangeSource struct {
	root         string
	subscription *fakeSubscription
}

func (f *fakeChangeSource) Subscribe(_ context.Context, root string) (filemodel.WorkspaceChangeSubscription, error) {
	f.root = root
	if f.subscription == nil {
		f.subscription = &fakeSubscription{events: make(chan filemodel.WorkspaceChange)}
	}
	return f.subscription, nil
}

func (f *fakeChangeSource) Close() error { return nil }

type fakeSubscription struct {
	events chan filemodel.WorkspaceChange
}

func (f *fakeSubscription) Events() <-chan filemodel.WorkspaceChange { return f.events }

func (f *fakeSubscription) Close() error { return nil }

func testConfig() Config {
	return Config{MaxTextBytes: 1, MaxDirectoryEntries: 1, MaxRecursiveDeleteEntries: 1, OperationTimeout: time.Second}
}

func serviceRoot(t *testing.T, service *Service, workspaceId string) string {
	t.Helper()
	operation, err := service.root(context.Background(), workspaceId)
	if err != nil {
		t.Fatalf("get workspace root: %v", err)
	}
	defer operation.Cancel()
	return operation.Root
}
