package application

import (
	"context"
	"fmt"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a WebTerminalAccess) ListFiles(ctx context.Context, request *agent.ListFilesReq) (*agent.ListFilesResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	result, err := a.Files.List(ctx, request.GetWorkspaceId(), request.GetPath())
	if err != nil {
		return nil, err
	}
	return &agent.ListFilesResp{Directory: fileEntry(result.Directory), Items: fileEntries(result.Items), Truncated: result.Truncated}, nil
}

func (a WebTerminalAccess) ReadFile(ctx context.Context, request *agent.ReadFileReq) (*agent.ReadFileResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	result, err := a.Files.Read(ctx, request.GetWorkspaceId(), request.GetPath())
	if err != nil {
		return nil, err
	}
	return &agent.ReadFileResp{Entry: fileEntry(result.Entry), Text: result.Text}, nil
}

func (a WebTerminalAccess) CreateFile(ctx context.Context, request *agent.CreateFileReq) (*agent.CreateFileResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	path, err := filemodel.ParseRelativePath(request.GetPath(), false)
	if err != nil {
		return nil, err
	}
	result, err := a.Files.CreateFile(ctx, request.GetWorkspaceId(), filemodel.CreateFileRequest{Path: path, Text: request.GetText(), Overwrite: request.GetOverwrite(), ExpectedParentRevision: request.GetExpectedParentRevision(), ExpectedDestinationRevision: request.GetExpectedDestinationRevision()})
	if err != nil {
		return nil, err
	}
	return &agent.CreateFileResp{Result: mutationResult(result)}, nil
}

func (a WebTerminalAccess) CreateDirectory(ctx context.Context, request *agent.CreateDirectoryReq) (*agent.CreateDirectoryResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	path, err := filemodel.ParseRelativePath(request.GetPath(), false)
	if err != nil {
		return nil, err
	}
	result, err := a.Files.CreateDirectory(ctx, request.GetWorkspaceId(), filemodel.CreateDirectoryRequest{Path: path, AllowExisting: request.GetAllowExisting(), ExpectedParentRevision: request.GetExpectedParentRevision()})
	if err != nil {
		return nil, err
	}
	return &agent.CreateDirectoryResp{Result: mutationResult(result)}, nil
}

func (a WebTerminalAccess) WriteFile(ctx context.Context, request *agent.WriteFileReq) (*agent.WriteFileResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	path, err := filemodel.ParseRelativePath(request.GetPath(), false)
	if err != nil {
		return nil, err
	}
	result, err := a.Files.Write(ctx, request.GetWorkspaceId(), filemodel.WriteRequest{Path: path, Text: request.GetText(), ExpectedRevision: request.GetExpectedRevision(), Force: request.GetForce()})
	if err != nil {
		return nil, err
	}
	return &agent.WriteFileResp{Result: mutationResult(result)}, nil
}

func (a WebTerminalAccess) RenameEntry(ctx context.Context, request *agent.RenameEntryReq) (*agent.RenameEntryResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	path, err := filemodel.ParseRelativePath(request.GetPath(), false)
	if err != nil {
		return nil, err
	}
	result, err := a.Files.Rename(ctx, request.GetWorkspaceId(), filemodel.RenameRequest{Path: path, NewName: request.GetNewName(), ExpectedSourceRevision: request.GetExpectedSourceRevision(), ExpectedParentRevision: request.GetExpectedParentRevision(), ExpectedDestinationRevision: request.GetExpectedDestinationRevision()})
	if err != nil {
		return nil, err
	}
	return &agent.RenameEntryResp{Result: mutationResult(result)}, nil
}

func (a WebTerminalAccess) MoveEntry(ctx context.Context, request *agent.MoveEntryReq) (*agent.MoveEntryResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	source, err := filemodel.ParseRelativePath(request.GetSourcePath(), false)
	if err != nil {
		return nil, err
	}
	destination, err := filemodel.ParseRelativePath(request.GetDestinationPath(), false)
	if err != nil {
		return nil, err
	}
	result, err := a.Files.Move(ctx, request.GetWorkspaceId(), filemodel.MoveRequest{SourcePath: source, DestinationPath: destination, ExpectedSourceRevision: request.GetExpectedSourceRevision(), ExpectedSourceParentRevision: request.GetExpectedSourceParentRevision(), ExpectedDestinationParentRevision: request.GetExpectedDestinationParentRevision(), ExpectedDestinationRevision: request.GetExpectedDestinationRevision()})
	if err != nil {
		return nil, err
	}
	return &agent.MoveEntryResp{Result: mutationResult(result)}, nil
}

func (a WebTerminalAccess) DeleteEntry(ctx context.Context, request *agent.DeleteEntryReq) (*agent.DeleteEntryResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	path, err := filemodel.ParseRelativePath(request.GetPath(), false)
	if err != nil {
		return nil, err
	}
	result, err := a.Files.Delete(ctx, request.GetWorkspaceId(), filemodel.DeleteRequest{Path: path, ExpectedRevision: request.GetExpectedRevision(), ExpectedParentRevision: request.GetExpectedParentRevision(), Recursive: request.GetRecursive()})
	if err != nil {
		return nil, err
	}
	return &agent.DeleteEntryResp{Result: mutationResult(result)}, nil
}

func (a WebTerminalAccess) GitStatus(ctx context.Context, workspaceId string) (*agent.GitStatusResp, error) {
	if err := a.gitService(); err != nil {
		return nil, err
	}
	result, err := a.Git.Status(ctx, workspaceId)
	if err != nil {
		return nil, err
	}
	return &agent.GitStatusResp{State: gitState(result.State), Changes: gitChanges(result.Changes), Message: result.Message}, nil
}

func (a WebTerminalAccess) GitDiff(ctx context.Context, request *agent.GitDiffReq) (*agent.GitDiffResp, error) {
	if err := a.gitService(); err != nil {
		return nil, err
	}
	result, err := a.Git.Diff(ctx, request.GetWorkspaceId(), gitmodel.DiffRequest{Path: request.GetPath(), Layer: gitLayerFromProto(request.GetLayer())})
	if err != nil {
		return nil, err
	}
	return &agent.GitDiffResp{State: gitState(result.State), OriginalPath: result.OriginalPath, ModifiedPath: result.ModifiedPath, OriginalText: result.OriginalText, ModifiedText: result.ModifiedText, Message: result.Message}, nil
}

func (a WebTerminalAccess) fileService() error {
	if a.Files == nil {
		return fmt.Errorf("file service is unavailable")
	}
	return nil
}
func (a WebTerminalAccess) gitService() error {
	if a.Git == nil {
		return fmt.Errorf("git service is unavailable")
	}
	return nil
}

func fileEntry(entry filemodel.Entry) *agent.FileEntry {
	result := &agent.FileEntry{Path: entry.Path.String(), Name: entry.Name, Size: entry.Size, ModifiedAt: timestamppb.New(entry.ModifiedAt), Revision: entry.Revision}
	if entry.Kind == filemodel.EntryKindDirectory {
		result.Kind = agent.FileEntryKind_FILE_ENTRY_KIND_DIRECTORY
	} else {
		result.Kind = agent.FileEntryKind_FILE_ENTRY_KIND_FILE
	}
	if entry.HasChildren != nil {
		result.HasChildren = entry.HasChildren
	}
	return result
}
func fileEntries(entries []filemodel.Entry) []*agent.FileEntry {
	result := make([]*agent.FileEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, fileEntry(entry))
	}
	return result
}
func mutationResult(value filemodel.MutationResult) *agent.FileMutationResult {
	result := &agent.FileMutationResult{AffectedCount: int32(value.AffectedCount)}
	if value.SourceEntry != nil {
		result.SourceEntry = fileEntry(*value.SourceEntry)
	}
	if value.DestinationEntry != nil {
		result.DestinationEntry = fileEntry(*value.DestinationEntry)
	}
	for _, prefix := range value.AffectedPathPrefixes {
		result.AffectedPathPrefixes = append(result.AffectedPathPrefixes, prefix.String())
	}
	return result
}
func gitState(value gitmodel.State) agent.GitState {
	switch value {
	case gitmodel.StateAvailable:
		return agent.GitState_GIT_STATE_AVAILABLE
	case gitmodel.StateNotRepository:
		return agent.GitState_GIT_STATE_NOT_REPOSITORY
	case gitmodel.StateExecutableUnavailable:
		return agent.GitState_GIT_STATE_EXECUTABLE_UNAVAILABLE
	case gitmodel.StateBinary:
		return agent.GitState_GIT_STATE_BINARY
	case gitmodel.StateTooLarge:
		return agent.GitState_GIT_STATE_TOO_LARGE
	case gitmodel.StateUnmerged:
		return agent.GitState_GIT_STATE_UNMERGED
	case gitmodel.StateSubmodule:
		return agent.GitState_GIT_STATE_SUBMODULE
	case gitmodel.StateLayerUnavailable:
		return agent.GitState_GIT_STATE_LAYER_UNAVAILABLE
	default:
		return agent.GitState_GIT_STATE_UNAVAILABLE
	}
}
func gitLayer(value gitmodel.Layer) agent.GitLayer {
	switch value {
	case gitmodel.LayerStaged:
		return agent.GitLayer_GIT_LAYER_STAGED
	case gitmodel.LayerUnstaged:
		return agent.GitLayer_GIT_LAYER_UNSTAGED
	case gitmodel.LayerUntracked:
		return agent.GitLayer_GIT_LAYER_UNTRACKED
	default:
		return agent.GitLayer_GIT_LAYER_UNSPECIFIED
	}
}
func gitLayerFromProto(value agent.GitLayer) gitmodel.Layer {
	switch value {
	case agent.GitLayer_GIT_LAYER_STAGED:
		return gitmodel.LayerStaged
	case agent.GitLayer_GIT_LAYER_UNSTAGED:
		return gitmodel.LayerUnstaged
	case agent.GitLayer_GIT_LAYER_UNTRACKED:
		return gitmodel.LayerUntracked
	default:
		return ""
	}
}
func gitChanges(changes []gitmodel.Change) []*agent.GitChange {
	result := make([]*agent.GitChange, 0, len(changes))
	for _, change := range changes {
		item := &agent.GitChange{Path: change.Path, OriginalPath: change.OriginalPath, IndexStatus: change.IndexStatus, WorktreeStatus: change.WorktreeStatus, Untracked: change.Untracked, Unmerged: change.Unmerged}
		for _, layer := range change.AvailableLayers {
			item.AvailableLayers = append(item.AvailableLayers, gitLayer(layer))
		}
		result = append(result, item)
	}
	return result
}
