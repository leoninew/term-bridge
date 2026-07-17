package application

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a WebTerminalAccess) FsStat(ctx context.Context, request *agent.FsStatReq) (*agent.FsStatResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	stat, err := a.fsStat(ctx, request.GetWorkspaceId(), request.GetPath())
	if err != nil {
		return nil, err
	}
	return &agent.FsStatResp{Stat: stat}, nil
}

func (a WebTerminalAccess) FsReadDirectory(ctx context.Context, request *agent.FsReadDirectoryReq) (*agent.FsReadDirectoryResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	result, err := a.Files.List(ctx, request.GetWorkspaceId(), request.GetPath())
	if err != nil {
		return nil, err
	}
	entries := make([]*agent.FsDirectoryEntry, 0, len(result.Items))
	for _, item := range result.Items {
		entries = append(entries, &agent.FsDirectoryEntry{Name: item.Name, Type: fileType(item.Kind)})
	}
	return &agent.FsReadDirectoryResp{Entries: entries, Truncated: result.Truncated}, nil
}

func (a WebTerminalAccess) FsReadFile(ctx context.Context, request *agent.FsReadFileReq) (*agent.FsReadFileResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	result, err := a.Files.Read(ctx, request.GetWorkspaceId(), request.GetPath())
	if err != nil {
		return nil, err
	}
	return &agent.FsReadFileResp{Content: []byte(result.Text), Stat: fileStat(result.Entry)}, nil
}

func (a WebTerminalAccess) FsWriteFile(ctx context.Context, request *agent.FsWriteFileReq) (*agent.FsWriteFileResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	pathValue, err := filemodel.ParseRelativePath(request.GetPath(), false)
	if err != nil {
		return nil, err
	}
	text, err := contentAsText(request.GetContent())
	if err != nil {
		return nil, err
	}

	// VS Code FileSystemProvider.writeFile contract:
	// - missing + !create  -> FileNotFound
	// - exists + !overwrite -> FileExists
	// - otherwise create or replace
	_, statErr := a.fsStat(ctx, request.GetWorkspaceId(), request.GetPath())
	exists := statErr == nil
	if !exists {
		if filemodel.CodeOf(statErr) != "file_not_found" {
			return nil, statErr
		}
		if !request.GetCreate() {
			return nil, filemodel.NewError("file_not_found", "The workspace entry was not found.")
		}
	} else if !request.GetOverwrite() {
		return nil, filemodel.NewError("already_exists", "The destination already exists.")
	}

	var result filemodel.MutationResult
	if exists {
		result, err = a.Files.Write(ctx, request.GetWorkspaceId(), filemodel.WriteRequest{
			Path:             pathValue,
			Text:             text,
			ExpectedRevision: request.GetEtag(),
			Force:            request.GetEtag() == "",
		})
	} else {
		result, err = a.Files.CreateFile(ctx, request.GetWorkspaceId(), filemodel.CreateFileRequest{
			Path:      pathValue,
			Text:      text,
			Overwrite: false,
		})
	}
	if err != nil {
		return nil, err
	}
	if result.DestinationEntry == nil {
		stat, statErr := a.fsStat(ctx, request.GetWorkspaceId(), request.GetPath())
		if statErr != nil {
			return nil, statErr
		}
		return &agent.FsWriteFileResp{Stat: stat}, nil
	}
	return &agent.FsWriteFileResp{Stat: fileStat(*result.DestinationEntry)}, nil
}

func (a WebTerminalAccess) FsCreateDirectory(ctx context.Context, request *agent.FsCreateDirectoryReq) (*agent.FsCreateDirectoryResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	pathValue, err := filemodel.ParseRelativePath(request.GetPath(), false)
	if err != nil {
		return nil, err
	}
	result, err := a.Files.CreateDirectory(ctx, request.GetWorkspaceId(), filemodel.CreateDirectoryRequest{
		Path:          pathValue,
		AllowExisting: false,
	})
	if err != nil {
		return nil, err
	}
	if result.DestinationEntry == nil {
		stat, statErr := a.fsStat(ctx, request.GetWorkspaceId(), request.GetPath())
		if statErr != nil {
			return nil, statErr
		}
		return &agent.FsCreateDirectoryResp{Stat: stat}, nil
	}
	return &agent.FsCreateDirectoryResp{Stat: fileStat(*result.DestinationEntry)}, nil
}

func (a WebTerminalAccess) FsDelete(ctx context.Context, request *agent.FsDeleteReq) (*agent.FsDeleteResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	pathValue, err := filemodel.ParseRelativePath(request.GetPath(), false)
	if err != nil {
		return nil, err
	}
	_, err = a.Files.Delete(ctx, request.GetWorkspaceId(), filemodel.DeleteRequest{
		Path:      pathValue,
		Recursive: request.GetRecursive(),
	})
	if err != nil {
		return nil, err
	}
	return &agent.FsDeleteResp{}, nil
}

func (a WebTerminalAccess) FsRename(ctx context.Context, request *agent.FsRenameReq) (*agent.FsRenameResp, error) {
	if err := a.fileService(); err != nil {
		return nil, err
	}
	oldPath, err := filemodel.ParseRelativePath(request.GetOldPath(), false)
	if err != nil {
		return nil, err
	}
	newPath, err := filemodel.ParseRelativePath(request.GetNewPath(), false)
	if err != nil {
		return nil, err
	}
	result, err := a.Files.Move(ctx, request.GetWorkspaceId(), filemodel.MoveRequest{
		SourcePath:      oldPath,
		DestinationPath: newPath,
		Overwrite:       request.GetOverwrite(),
	})
	if err != nil {
		return nil, err
	}
	if result.DestinationEntry == nil {
		stat, statErr := a.fsStat(ctx, request.GetWorkspaceId(), request.GetNewPath())
		if statErr != nil {
			return nil, statErr
		}
		return &agent.FsRenameResp{Stat: stat}, nil
	}
	return &agent.FsRenameResp{Stat: fileStat(*result.DestinationEntry)}, nil
}

func (a WebTerminalAccess) ScmStatus(ctx context.Context, request *agent.ScmStatusReq) (*agent.ScmStatusResp, error) {
	if err := a.gitService(); err != nil {
		return nil, err
	}
	status, err := a.Git.Status(ctx, request.GetWorkspaceId())
	if err != nil {
		return nil, err
	}
	return scmStatusFromModel(status), nil
}

func (a WebTerminalAccess) ScmOriginalContent(ctx context.Context, request *agent.ScmOriginalContentReq) (*agent.ScmOriginalContentResp, error) {
	if err := a.gitService(); err != nil {
		return nil, err
	}
	layer := groupToLayer(request.GetGroupId())
	if layer == "" {
		return &agent.ScmOriginalContentResp{ResourceState: agent.ScmResourceState_SCM_RESOURCE_STATE_UNAVAILABLE, Message: "Unknown SCM group."}, nil
	}
	diff, err := a.Git.Diff(ctx, request.GetWorkspaceId(), gitmodel.DiffRequest{Path: request.GetPath(), Layer: layer})
	if err != nil {
		return nil, err
	}
	return &agent.ScmOriginalContentResp{
		ResourceState: scmResourceStateFromGit(diff.State),
		Content:       []byte(diff.OriginalText),
		Message:       diff.Message,
	}, nil
}

func (a WebTerminalAccess) ScmExecute(ctx context.Context, request *agent.ScmExecuteReq) (*agent.ScmExecuteResp, error) {
	if err := a.gitService(); err != nil {
		return nil, err
	}
	switch request.GetCommand() {
	case agent.ScmCommand_SCM_COMMAND_STAGE, agent.ScmCommand_SCM_COMMAND_UNSTAGE, agent.ScmCommand_SCM_COMMAND_DISCARD:
		mutation, layer := scmCommandToMutation(request.GetCommand(), request.GetGroupId())
		if mutation == "" {
			return &agent.ScmExecuteResp{OperationState: agent.ScmOperationState_SCM_OPERATION_STATE_UNAVAILABLE, Message: "Unsupported SCM command."}, nil
		}
		result, err := a.Git.MutatePath(ctx, request.GetWorkspaceId(), gitmodel.PathMutationRequest{
			Path:     request.GetPath(),
			Layer:    layer,
			Mutation: mutation,
		})
		if err != nil {
			return nil, err
		}
		return &agent.ScmExecuteResp{
			OperationState: scmOperationState(result.State),
			Message:        result.Message,
			Status:         scmStatusFromModel(result.Status),
		}, nil
	case agent.ScmCommand_SCM_COMMAND_COMMIT:
		result, err := a.Git.Commit(ctx, request.GetWorkspaceId(), gitmodel.CommitRequest{Message: request.GetMessage()})
		if err != nil {
			return nil, err
		}
		return &agent.ScmExecuteResp{
			OperationState: scmOperationState(result.State),
			Message:        result.Message,
			Status:         scmStatusFromModel(result.Status),
			Repository:     scmRepositoryFromModel(result.Summary),
		}, nil
	case agent.ScmCommand_SCM_COMMAND_CREATE_BRANCH:
		result, err := a.Git.CreateBranch(ctx, request.GetWorkspaceId(), gitmodel.BranchRequest{Name: request.GetBranchName()})
		if err != nil {
			return nil, err
		}
		return &agent.ScmExecuteResp{
			OperationState: scmOperationState(result.State),
			Message:        result.Message,
			Repository:     scmRepositoryFromModel(result.Summary),
		}, nil
	case agent.ScmCommand_SCM_COMMAND_SWITCH_BRANCH:
		result, err := a.Git.SwitchBranch(ctx, request.GetWorkspaceId(), gitmodel.BranchRequest{Name: request.GetBranchName()})
		if err != nil {
			return nil, err
		}
		return &agent.ScmExecuteResp{
			OperationState: scmOperationState(result.State),
			Message:        result.Message,
			Status:         scmStatusFromModel(result.Status),
			Repository:     scmRepositoryFromModel(result.Summary),
		}, nil
	default:
		return &agent.ScmExecuteResp{OperationState: agent.ScmOperationState_SCM_OPERATION_STATE_UNAVAILABLE, Message: "Unknown SCM command."}, nil
	}
}

func (a WebTerminalAccess) ScmRepository(ctx context.Context, request *agent.ScmRepositoryReq) (*agent.ScmRepositoryResp, error) {
	if err := a.gitService(); err != nil {
		return nil, err
	}
	summary, err := a.Git.Summary(ctx, request.GetWorkspaceId())
	if err != nil {
		return nil, err
	}
	return scmRepositoryFromModel(summary), nil
}

func (a WebTerminalAccess) fsStat(ctx context.Context, workspaceId string, pathValue string) (*agent.FileStat, error) {
	list, err := a.Files.List(ctx, workspaceId, pathValue)
	if err == nil {
		return fileStat(list.Directory), nil
	}
	read, readErr := a.Files.Read(ctx, workspaceId, pathValue)
	if readErr != nil {
		return nil, err
	}
	return fileStat(read.Entry), nil
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

func contentAsText(content []byte) (string, error) {
	// Workbench FS is a text-editor filesystem: wire bytes must be UTF-8 without NUL.
	// Keep codes aligned with workspacefile store (file_not_text) for provider mapping.
	if !utf8.Valid(content) {
		return "", filemodel.NewError("file_not_text", "Only UTF-8 text content is supported.")
	}
	for _, b := range content {
		if b == 0 {
			return "", filemodel.NewError("file_not_text", "Only UTF-8 text content is supported.")
		}
	}
	return string(content), nil
}

func fileType(kind filemodel.EntryKind) agent.FileType {
	if kind == filemodel.EntryKindDirectory {
		return agent.FileType_FILE_TYPE_DIRECTORY
	}
	return agent.FileType_FILE_TYPE_FILE
}

func fileStat(entry filemodel.Entry) *agent.FileStat {
	return &agent.FileStat{
		Type:  fileType(entry.Kind),
		Ctime: entry.ModifiedAt.UnixMilli(),
		Mtime: entry.ModifiedAt.UnixMilli(),
		Size:  entry.Size,
		Etag:  entry.Revision,
	}
}

func scmStatusFromModel(status gitmodel.StatusResult) *agent.ScmStatusResp {
	if status.State != gitmodel.StateAvailable {
		return &agent.ScmStatusResp{State: scmState(status.State), Message: status.Message}
	}
	staged := &agent.ScmResourceGroup{Id: "staged", Label: "Staged Changes", HideWhenEmpty: true}
	changes := &agent.ScmResourceGroup{Id: "changes", Label: "Changes", HideWhenEmpty: true}
	untracked := &agent.ScmResourceGroup{Id: "untracked", Label: "Untracked", HideWhenEmpty: true}
	count := int32(0)
	for _, change := range status.Changes {
		for _, layer := range change.AvailableLayers {
			resource := &agent.ScmResource{
				Path:          change.Path,
				OriginalPath:  change.OriginalPath,
				Decorations:   scmDecorations(change, layer),
				ResourceState: agent.ScmResourceState_SCM_RESOURCE_STATE_AVAILABLE,
			}
			switch layer {
			case gitmodel.LayerStaged:
				resource.GroupId = "staged"
				staged.Resources = append(staged.Resources, resource)
			case gitmodel.LayerUnstaged:
				resource.GroupId = "changes"
				changes.Resources = append(changes.Resources, resource)
			case gitmodel.LayerUntracked:
				resource.GroupId = "untracked"
				untracked.Resources = append(untracked.Resources, resource)
			}
			count++
		}
	}
	return &agent.ScmStatusResp{
		State:   agent.ScmState_SCM_STATE_AVAILABLE,
		Groups:  []*agent.ScmResourceGroup{staged, changes, untracked},
		Count:   count,
		Message: status.Message,
	}
}

func scmDecorations(change gitmodel.Change, layer gitmodel.Layer) *agent.ScmResourceDecorations {
	letter := change.WorktreeStatus
	if layer == gitmodel.LayerStaged {
		letter = change.IndexStatus
	}
	if change.Untracked {
		letter = "U"
	}
	tooltip := strings.TrimSpace(change.IndexStatus + change.WorktreeStatus)
	return &agent.ScmResourceDecorations{
		StrikeThrough: letter == "D" || strings.Contains(change.WorktreeStatus, "D"),
		Tooltip:       tooltip,
		Letter:        letter,
	}
}

func scmState(value gitmodel.State) agent.ScmState {
	switch value {
	case gitmodel.StateAvailable:
		return agent.ScmState_SCM_STATE_AVAILABLE
	case gitmodel.StateNotRepository:
		return agent.ScmState_SCM_STATE_NOT_REPOSITORY
	case gitmodel.StateExecutableUnavailable:
		return agent.ScmState_SCM_STATE_EXECUTABLE_UNAVAILABLE
	default:
		return agent.ScmState_SCM_STATE_UNAVAILABLE
	}
}

func scmResourceStateFromGit(value gitmodel.State) agent.ScmResourceState {
	switch value {
	case gitmodel.StateAvailable:
		return agent.ScmResourceState_SCM_RESOURCE_STATE_AVAILABLE
	case gitmodel.StateBinary:
		return agent.ScmResourceState_SCM_RESOURCE_STATE_BINARY
	case gitmodel.StateTooLarge:
		return agent.ScmResourceState_SCM_RESOURCE_STATE_TOO_LARGE
	case gitmodel.StateUnmerged:
		return agent.ScmResourceState_SCM_RESOURCE_STATE_UNMERGED
	case gitmodel.StateSubmodule:
		return agent.ScmResourceState_SCM_RESOURCE_STATE_SUBMODULE
	default:
		return agent.ScmResourceState_SCM_RESOURCE_STATE_UNAVAILABLE
	}
}

func groupToLayer(groupId string) gitmodel.Layer {
	switch groupId {
	case "staged":
		return gitmodel.LayerStaged
	case "changes":
		return gitmodel.LayerUnstaged
	case "untracked":
		return gitmodel.LayerUntracked
	default:
		return ""
	}
}

func scmCommandToMutation(command agent.ScmCommand, groupId string) (gitmodel.MutationKind, gitmodel.Layer) {
	switch command {
	case agent.ScmCommand_SCM_COMMAND_STAGE:
		return gitmodel.MutationStage, groupToLayer(groupId)
	case agent.ScmCommand_SCM_COMMAND_UNSTAGE:
		return gitmodel.MutationUnstage, gitmodel.LayerStaged
	case agent.ScmCommand_SCM_COMMAND_DISCARD:
		if groupId == "untracked" {
			return gitmodel.MutationDeleteUntracked, gitmodel.LayerUntracked
		}
		return gitmodel.MutationRestoreUnstaged, gitmodel.LayerUnstaged
	default:
		return "", ""
	}
}

func scmOperationState(value gitmodel.OperationState) agent.ScmOperationState {
	switch value {
	case gitmodel.OperationStateAvailable:
		return agent.ScmOperationState_SCM_OPERATION_STATE_OK
	case gitmodel.OperationStateCleanWorktreeRequired:
		return agent.ScmOperationState_SCM_OPERATION_STATE_CLEAN_WORKTREE_REQUIRED
	case gitmodel.OperationStateNoStagedChanges:
		return agent.ScmOperationState_SCM_OPERATION_STATE_NO_STAGED_CHANGES
	case gitmodel.OperationStateBranchExists:
		return agent.ScmOperationState_SCM_OPERATION_STATE_BRANCH_EXISTS
	case gitmodel.OperationStateBranchNotFound:
		return agent.ScmOperationState_SCM_OPERATION_STATE_BRANCH_NOT_FOUND
	case gitmodel.OperationStateInvalidBranchName:
		return agent.ScmOperationState_SCM_OPERATION_STATE_INVALID_BRANCH_NAME
	case gitmodel.OperationStateInvalidCommitMessage:
		return agent.ScmOperationState_SCM_OPERATION_STATE_INVALID_COMMIT_MESSAGE
	case gitmodel.OperationStateConflict:
		return agent.ScmOperationState_SCM_OPERATION_STATE_CONFLICT
	case gitmodel.OperationStateCommitFailed:
		return agent.ScmOperationState_SCM_OPERATION_STATE_FAILED
	default:
		return agent.ScmOperationState_SCM_OPERATION_STATE_UNAVAILABLE
	}
}

func scmRepositoryFromModel(summary gitmodel.RepositorySummary) *agent.ScmRepositoryResp {
	if summary.State != gitmodel.StateAvailable && summary.State != "" {
		return &agent.ScmRepositoryResp{State: scmState(summary.State), Message: summary.Message}
	}
	history := make([]*agent.ScmHistoryEntry, 0, len(summary.History))
	for _, entry := range summary.History {
		item := &agent.ScmHistoryEntry{
			Id:         entry.Id,
			ShortId:    entry.ShortId,
			Subject:    entry.Subject,
			AuthorName: entry.AuthorName,
		}
		if !entry.AuthoredAt.IsZero() {
			item.AuthoredAt = timestamppb.New(entry.AuthoredAt)
		}
		history = append(history, item)
	}
	return &agent.ScmRepositoryResp{
		State:         agent.ScmState_SCM_STATE_AVAILABLE,
		CurrentBranch: summary.CurrentBranch,
		LocalBranches: summary.LocalBranches,
		History:       history,
		Message:       summary.Message,
	}
}
