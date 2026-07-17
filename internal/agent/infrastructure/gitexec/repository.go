package gitexec

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	workspacefile "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/storage/workspacefile"
	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
)

type Config struct {
	Executable     string
	MaxStdoutBytes int64
	MaxStderrBytes int64
	MaxTextBytes   int64
}

type worktreeStore interface {
	Read(ctx context.Context, root string, path filemodel.RelativePath) (filemodel.ReadResult, error)
	DeleteRegularFile(ctx context.Context, root string, path filemodel.RelativePath) error
}

type Repository struct {
	config   Config
	files    worktreeStore
	lookPath func(string) (string, error)
	environ  func() []string
}

func New(config Config, files *workspacefile.Store) *Repository {
	return &Repository{config: config, files: files, lookPath: exec.LookPath, environ: os.Environ}
}

func (r *Repository) Identity(ctx context.Context, root string) (gitmodel.RepositoryIdentity, error) {
	if state := r.workTreeState(ctx, root); state != gitmodel.StateAvailable {
		return gitmodel.RepositoryIdentity{State: state}, nil
	}
	topLevel, state := r.run(ctx, root, "rev-parse", "--show-toplevel")
	if state != gitmodel.StateAvailable {
		return gitmodel.RepositoryIdentity{State: state}, nil
	}
	canonicalRoot, err := filepath.EvalSymlinks(strings.TrimSpace(string(topLevel)))
	if err != nil {
		return gitmodel.RepositoryIdentity{State: gitmodel.StateUnavailable}, nil
	}
	return gitmodel.RepositoryIdentity{State: gitmodel.StateAvailable, Key: filepath.Clean(canonicalRoot)}, nil
}

func (r *Repository) Status(ctx context.Context, root string) (gitmodel.StatusResult, error) {
	if state := r.workTreeState(ctx, root); state != gitmodel.StateAvailable {
		return gitmodel.StatusResult{State: state}, nil
	}
	topLevel, state := r.run(ctx, root, "rev-parse", "--show-toplevel")
	if state != gitmodel.StateAvailable {
		return gitmodel.StatusResult{State: state}, nil
	}
	workspacePrefix, err := workspacePrefix(root, strings.TrimSpace(string(topLevel)))
	if err != nil {
		return gitmodel.StatusResult{State: gitmodel.StateUnavailable}, nil
	}
	output, state := r.run(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if state != gitmodel.StateAvailable {
		return gitmodel.StatusResult{State: state}, nil
	}
	changes, err := parseStatus(output)
	if err != nil {
		return gitmodel.StatusResult{State: gitmodel.StateUnavailable}, nil
	}
	changes = restrictToWorkspace(changes, workspacePrefix)
	return gitmodel.StatusResult{State: gitmodel.StateAvailable, Changes: changes}, nil
}

func (r *Repository) Diff(ctx context.Context, root string, request gitmodel.DiffRequest) (gitmodel.DiffResult, error) {
	status, err := r.Status(ctx, root)
	if err != nil || status.State != gitmodel.StateAvailable {
		return gitmodel.DiffResult{State: status.State}, err
	}
	change, ok := findChange(status.Changes, request.Path)
	if !ok {
		return gitmodel.DiffResult{State: gitmodel.StateLayerUnavailable}, nil
	}
	if change.Unmerged {
		return gitmodel.DiffResult{State: gitmodel.StateUnmerged}, nil
	}
	if !hasLayer(change, request.Layer) {
		return gitmodel.DiffResult{State: gitmodel.StateLayerUnavailable}, nil
	}
	if err := validateGitPath(request.Path); err != nil {
		return gitmodel.DiffResult{State: gitmodel.StateUnavailable}, nil
	}
	prefix, state := r.workspacePrefix(ctx, root)
	if state != gitmodel.StateAvailable {
		return gitmodel.DiffResult{State: state}, nil
	}
	return r.diff(ctx, root, change, request.Layer, prefix)
}

func (r *Repository) Summary(ctx context.Context, root string) (gitmodel.RepositorySummary, error) {
	if state := r.workTreeState(ctx, root); state != gitmodel.StateAvailable {
		return gitmodel.RepositorySummary{State: state}, nil
	}
	branchOutput, state := r.run(ctx, root, "branch", "--show-current")
	if state != gitmodel.StateAvailable {
		return gitmodel.RepositorySummary{State: state}, nil
	}
	branchesOutput, state := r.run(ctx, root, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if state != gitmodel.StateAvailable {
		return gitmodel.RepositorySummary{State: state}, nil
	}
	hasHead, state := r.commandSucceeded(ctx, root, "rev-parse", "--verify", "HEAD")
	if state != gitmodel.StateAvailable {
		return gitmodel.RepositorySummary{State: state}, nil
	}
	var history []gitmodel.HistoryEntry
	if hasHead {
		historyOutput, state := r.run(ctx, root, "log", "-z", "-n", strconv.Itoa(gitmodel.MaxHistoryEntries), "--format=%H%x00%h%x00%s%x00%an%x00%aI")
		if state != gitmodel.StateAvailable {
			return gitmodel.RepositorySummary{State: state}, nil
		}
		parsedHistory, err := parseHistory(historyOutput)
		if err != nil {
			return gitmodel.RepositorySummary{State: gitmodel.StateUnavailable}, nil
		}
		history = parsedHistory
	}
	return gitmodel.RepositorySummary{
		State:         gitmodel.StateAvailable,
		CurrentBranch: strings.TrimSpace(string(branchOutput)),
		LocalBranches: splitNonEmptyLines(branchesOutput),
		History:       history,
	}, nil
}

func (r *Repository) MutatePath(ctx context.Context, root string, request gitmodel.PathMutationRequest) (gitmodel.OperationResult, error) {
	status, err := r.Status(ctx, root)
	if err != nil || status.State != gitmodel.StateAvailable {
		return operationForGitState(status.State), err
	}
	change, ok := findChange(status.Changes, request.Path)
	if !ok || change.Unmerged || !hasLayer(change, request.Layer) || !validPathMutation(request) {
		return gitmodel.OperationResult{State: gitmodel.OperationStateConflict, Status: status, Message: "Git status changed. Refresh, then retry the operation."}, nil
	}
	if err := validateGitPath(request.Path); err != nil {
		return gitmodel.OperationResult{State: gitmodel.OperationStateConflict, Status: status, Message: "Git status changed. Refresh, then retry the operation."}, nil
	}
	path, state := r.workspacePathArgument(ctx, root, request.Path)
	if state != gitmodel.StateAvailable {
		return operationForGitState(state), nil
	}
	switch request.Mutation {
	case gitmodel.MutationStage:
		if _, state := r.run(ctx, root, "add", "--", path); state != gitmodel.StateAvailable {
			return operationForGitState(state), nil
		}
	case gitmodel.MutationUnstage:
		if change.IndexStatus == "A" {
			if _, state := r.run(ctx, root, "rm", "--cached", "--", path); state != gitmodel.StateAvailable {
				return operationForGitState(state), nil
			}
		} else if _, state := r.run(ctx, root, "restore", "--staged", "--", path); state != gitmodel.StateAvailable {
			return operationForGitState(state), nil
		}
	case gitmodel.MutationRestoreUnstaged:
		if _, state := r.run(ctx, root, "restore", "--worktree", "--", path); state != gitmodel.StateAvailable {
			return operationForGitState(state), nil
		}
	case gitmodel.MutationDeleteUntracked:
		if r.files == nil {
			return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable}, nil
		}
		parsed, err := filemodel.ParseRelativePath(path, false)
		if err != nil {
			return gitmodel.OperationResult{State: gitmodel.OperationStateConflict, Status: status, Message: "Git status changed. Refresh, then retry the operation."}, nil
		}
		if err := r.files.DeleteRegularFile(ctx, root, parsed); err != nil {
			return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable}, nil
		}
	}
	fresh, err := r.Status(ctx, root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	return gitmodel.OperationResult{State: gitmodel.OperationStateAvailable, Status: fresh}, nil
}

func (r *Repository) Commit(ctx context.Context, root string, request gitmodel.CommitRequest) (gitmodel.OperationResult, error) {
	if err := gitmodel.ValidateCommitMessage(request.Message); err != nil {
		return gitmodel.OperationResult{State: gitmodel.OperationStateInvalidCommitMessage, Message: "The commit message is invalid."}, nil
	}
	status, err := r.Status(ctx, root)
	if err != nil || status.State != gitmodel.StateAvailable {
		return operationForGitState(status.State), err
	}
	if !hasStagedChanges(status.Changes) {
		return gitmodel.OperationResult{State: gitmodel.OperationStateNoStagedChanges, Status: status, Message: "There are no staged changes to commit."}, nil
	}
	outsideStaged, state := r.hasStagedOutsideWorkspace(ctx, root)
	if state != gitmodel.StateAvailable {
		return operationForGitState(state), nil
	}
	if outsideStaged {
		return gitmodel.OperationResult{State: gitmodel.OperationStateConflict, Status: status, Message: "The repository has staged changes outside this workspace."}, nil
	}
	messageFile, err := os.CreateTemp("", "termbridge-git-commit-*.txt")
	if err != nil {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable}, nil
	}
	messagePath := messageFile.Name()
	defer func() { _ = os.Remove(messagePath) }()
	if err := messageFile.Chmod(0o600); err != nil {
		_ = messageFile.Close()
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable}, nil
	}
	if _, err := messageFile.WriteString(request.Message); err != nil {
		_ = messageFile.Close()
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable}, nil
	}
	if err := messageFile.Close(); err != nil {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable}, nil
	}
	commit := r.runProcess(ctx, root, "commit", "--file="+messagePath)
	if commit.state != gitmodel.StateAvailable {
		return operationForGitState(commit.state), nil
	}
	if commit.err != nil {
		return gitmodel.OperationResult{State: gitmodel.OperationStateCommitFailed, Message: "Git could not create the commit."}, nil
	}
	fresh, err := r.Status(ctx, root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	summary, err := r.Summary(ctx, root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	return gitmodel.OperationResult{State: gitmodel.OperationStateAvailable, Status: fresh, Summary: summary}, nil
}

func (r *Repository) CreateBranch(ctx context.Context, root string, request gitmodel.BranchRequest) (gitmodel.OperationResult, error) {
	if !validBranchName(request.Name) {
		return gitmodel.OperationResult{State: gitmodel.OperationStateInvalidBranchName, Message: "The branch name is invalid."}, nil
	}
	if state := r.checkBranchName(ctx, root, request.Name); state != gitmodel.StateAvailable {
		return operationForGitState(state), nil
	}
	exists, state := r.commandSucceeded(ctx, root, "show-ref", "--verify", "--quiet", "refs/heads/"+request.Name)
	if state != gitmodel.StateAvailable {
		return operationForGitState(state), nil
	}
	if exists {
		return gitmodel.OperationResult{State: gitmodel.OperationStateBranchExists, Message: "The branch already exists."}, nil
	}
	hasHead, state := r.commandSucceeded(ctx, root, "rev-parse", "--verify", "HEAD")
	if state != gitmodel.StateAvailable {
		return operationForGitState(state), nil
	}
	if !hasHead {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Message: "Git cannot create a branch before the first commit."}, nil
	}
	if _, state := r.run(ctx, root, "branch", "--", request.Name); state != gitmodel.StateAvailable {
		return operationForGitState(state), nil
	}
	summary, err := r.Summary(ctx, root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	return gitmodel.OperationResult{State: gitmodel.OperationStateAvailable, Summary: summary}, nil
}

func (r *Repository) SwitchBranch(ctx context.Context, root string, request gitmodel.BranchRequest) (gitmodel.OperationResult, error) {
	if !validBranchName(request.Name) {
		return gitmodel.OperationResult{State: gitmodel.OperationStateInvalidBranchName, Message: "The branch name is invalid."}, nil
	}
	if state := r.checkBranchName(ctx, root, request.Name); state != gitmodel.StateAvailable {
		return operationForGitState(state), nil
	}
	if r.repositoryDirty(ctx, root) {
		return gitmodel.OperationResult{State: gitmodel.OperationStateCleanWorktreeRequired, Message: "The repository has uncommitted changes."}, nil
	}
	exists, state := r.commandSucceeded(ctx, root, "show-ref", "--verify", "--quiet", "refs/heads/"+request.Name)
	if state != gitmodel.StateAvailable {
		return operationForGitState(state), nil
	}
	if !exists {
		return gitmodel.OperationResult{State: gitmodel.OperationStateBranchNotFound, Message: "The local branch was not found."}, nil
	}
	if _, state := r.run(ctx, root, "switch", "--", request.Name); state != gitmodel.StateAvailable {
		return operationForGitState(state), nil
	}
	fresh, err := r.Status(ctx, root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	summary, err := r.Summary(ctx, root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	return gitmodel.OperationResult{State: gitmodel.OperationStateAvailable, Status: fresh, Summary: summary}, nil
}

func validPathMutation(request gitmodel.PathMutationRequest) bool {
	switch request.Mutation {
	case gitmodel.MutationStage:
		return request.Layer == gitmodel.LayerUnstaged || request.Layer == gitmodel.LayerUntracked
	case gitmodel.MutationUnstage:
		return request.Layer == gitmodel.LayerStaged
	case gitmodel.MutationRestoreUnstaged:
		return request.Layer == gitmodel.LayerUnstaged
	case gitmodel.MutationDeleteUntracked:
		return request.Layer == gitmodel.LayerUntracked
	default:
		return false
	}
}

func operationForGitState(state gitmodel.State) gitmodel.OperationResult {
	return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Status: gitmodel.StatusResult{State: state}}
}

func hasStagedChanges(changes []gitmodel.Change) bool {
	return slices.ContainsFunc(changes, func(change gitmodel.Change) bool {
		return hasLayer(change, gitmodel.LayerStaged)
	})
}

func (r *Repository) hasStagedOutsideWorkspace(ctx context.Context, root string) (bool, gitmodel.State) {
	prefix, state := r.workspacePrefix(ctx, root)
	if state != gitmodel.StateAvailable {
		return false, state
	}
	output, state := r.run(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=no")
	if state != gitmodel.StateAvailable {
		return false, state
	}
	changes, err := parseStatus(output)
	if err != nil {
		return false, gitmodel.StateUnavailable
	}
	for _, change := range changes {
		if !hasLayer(change, gitmodel.LayerStaged) {
			continue
		}
		if _, scoped := workspaceRelativePath(change.Path, prefix); !scoped {
			return true, gitmodel.StateAvailable
		}
		if change.OriginalPath != "" {
			if _, scoped := workspaceRelativePath(change.OriginalPath, prefix); !scoped {
				return true, gitmodel.StateAvailable
			}
		}
	}
	return false, gitmodel.StateAvailable
}

func (r *Repository) repositoryDirty(ctx context.Context, root string) bool {
	output, state := r.run(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	return state != gitmodel.StateAvailable || len(output) > 0
}

func (r *Repository) checkBranchName(ctx context.Context, root string, name string) gitmodel.State {
	result := r.runProcess(ctx, root, "check-ref-format", "--branch", name)
	if result.state != gitmodel.StateAvailable || result.err != nil {
		return gitmodel.StateUnavailable
	}
	return gitmodel.StateAvailable
}

func (r *Repository) commandSucceeded(ctx context.Context, root string, args ...string) (bool, gitmodel.State) {
	result := r.runProcess(ctx, root, args...)
	if result.state != gitmodel.StateAvailable {
		return false, result.state
	}
	return result.err == nil, gitmodel.StateAvailable
}

func validBranchName(value string) bool {
	return gitmodel.ValidateBranchName(value) == nil
}

func splitNonEmptyLines(output []byte) []string {
	var values []string
	for value := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func parseHistory(output []byte) ([]gitmodel.HistoryEntry, error) {
	values := bytes.Split(output, []byte{0})
	if len(values) == 1 && len(values[0]) == 0 {
		return nil, nil
	}
	if len(values)%5 != 1 || len(values[len(values)-1]) != 0 {
		return nil, errors.New("invalid Git history output")
	}
	entries := make([]gitmodel.HistoryEntry, 0, len(values)/5)
	for index := 0; index < len(values)-1; index += 5 {
		authoredAt, err := time.Parse(time.RFC3339, string(values[index+4]))
		if err != nil || values[index][0] == 0 || values[index+1][0] == 0 {
			return nil, errors.New("invalid Git history record")
		}
		entries = append(entries, gitmodel.HistoryEntry{
			Id:         string(values[index]),
			ShortId:    string(values[index+1]),
			Subject:    string(values[index+2]),
			AuthorName: string(values[index+3]),
			AuthoredAt: authoredAt,
		})
	}
	return entries, nil
}

func (r *Repository) diff(ctx context.Context, root string, change gitmodel.Change, layer gitmodel.Layer, prefix string) (gitmodel.DiffResult, error) {
	contentPath := indexContentPath(change, layer)
	repositoryContentPath := repositoryPath(prefix, contentPath)
	repositoryModifiedPath := repositoryPath(prefix, change.Path)
	if state := r.layerSubmoduleState(ctx, root, change, layer, repositoryContentPath, repositoryModifiedPath); state != gitmodel.StateAvailable {
		return gitmodel.DiffResult{State: state}, nil
	}
	result := gitmodel.DiffResult{State: gitmodel.StateAvailable, OriginalPath: contentPath, ModifiedPath: change.Path}
	var original []byte
	var modified []byte
	var state gitmodel.State
	switch layer {
	case gitmodel.LayerStaged:
		if change.IndexStatus == "A" {
			state = gitmodel.StateAvailable
		} else {
			original, state = r.show(ctx, root, "HEAD:"+repositoryContentPath)
		}
		if state != gitmodel.StateAvailable {
			return gitmodel.DiffResult{State: state}, nil
		}
		if change.IndexStatus == "D" {
			state = gitmodel.StateAvailable
		} else {
			modified, state = r.show(ctx, root, ":"+repositoryModifiedPath)
		}
	case gitmodel.LayerUnstaged:
		if change.WorktreeStatus == "A" || change.IndexStatus == "D" {
			state = gitmodel.StateAvailable
		} else {
			original, state = r.show(ctx, root, ":"+repositoryContentPath)
		}
		if state != gitmodel.StateAvailable {
			return gitmodel.DiffResult{State: state}, nil
		}
		if change.WorktreeStatus == "D" {
			state = gitmodel.StateAvailable
		} else {
			modified, state = r.readWorktree(ctx, root, change.Path)
		}
	case gitmodel.LayerUntracked:
		modified, state = r.readWorktree(ctx, root, change.Path)
	}
	if state != gitmodel.StateAvailable {
		return gitmodel.DiffResult{State: state}, nil
	}
	if r.exceedsTextLimit(original) || r.exceedsTextLimit(modified) {
		return gitmodel.DiffResult{State: gitmodel.StateTooLarge}, nil
	}
	if !text(original) || !text(modified) {
		return gitmodel.DiffResult{State: gitmodel.StateBinary}, nil
	}
	result.OriginalText, result.ModifiedText = string(original), string(modified)
	return result, nil
}

func indexContentPath(change gitmodel.Change, layer gitmodel.Layer) string {
	if layer == gitmodel.LayerUnstaged &&
		change.WorktreeStatus != "R" &&
		change.WorktreeStatus != "C" {
		return change.Path
	}
	if change.OriginalPath != "" {
		return change.OriginalPath
	}
	return change.Path
}

func (r *Repository) layerSubmoduleState(
	ctx context.Context,
	root string,
	change gitmodel.Change,
	layer gitmodel.Layer,
	repositoryContentPath string,
	repositoryModifiedPath string,
) gitmodel.State {
	switch layer {
	case gitmodel.LayerStaged:
		if change.IndexStatus != "A" {
			if state := r.objectSubmoduleState(ctx, root, "HEAD:"+repositoryContentPath); state != gitmodel.StateAvailable {
				return state
			}
		}
		if change.IndexStatus != "D" {
			return r.objectSubmoduleState(ctx, root, ":"+repositoryModifiedPath)
		}
	case gitmodel.LayerUnstaged:
		if change.WorktreeStatus != "A" && change.IndexStatus != "D" {
			if state := r.objectSubmoduleState(ctx, root, ":"+repositoryContentPath); state != gitmodel.StateAvailable {
				return state
			}
		}
	}
	return gitmodel.StateAvailable
}

func (r *Repository) objectSubmoduleState(ctx context.Context, root string, object string) gitmodel.State {
	output, state := r.run(ctx, root, "cat-file", "-t", object)
	if state != gitmodel.StateAvailable {
		return state
	}
	if strings.TrimSpace(string(output)) == "commit" {
		return gitmodel.StateSubmodule
	}
	return gitmodel.StateAvailable
}

func (r *Repository) show(ctx context.Context, root string, object string) ([]byte, gitmodel.State) {
	output, state := r.run(ctx, root, "show", "--no-ext-diff", "--no-textconv", object, "--")
	return output, state
}

const nonRepositoryExitCode = 128

func (r *Repository) workTreeState(ctx context.Context, root string) gitmodel.State {
	result := r.runProcess(ctx, root, "rev-parse", "--is-inside-work-tree")
	if result.state != gitmodel.StateAvailable {
		return result.state
	}
	if result.err != nil {
		var exitError *exec.ExitError
		if errors.As(result.err, &exitError) &&
			exitError.ExitCode() == nonRepositoryExitCode &&
			len(bytes.TrimSpace(result.output)) == 0 {
			hasMetadata, err := hasGitMetadata(root)
			if err != nil || hasMetadata {
				return gitmodel.StateUnavailable
			}
			return r.nonRepositoryState(ctx, root)
		}
		return gitmodel.StateUnavailable
	}
	if strings.TrimSpace(string(result.output)) != "true" {
		return gitmodel.StateNotRepository
	}
	return gitmodel.StateAvailable
}

func (r *Repository) nonRepositoryState(ctx context.Context, root string) gitmodel.State {
	result := r.runProcess(ctx, root, "config", "--list", "--includes")
	if result.state != gitmodel.StateAvailable || result.err != nil {
		return gitmodel.StateUnavailable
	}
	return gitmodel.StateNotRepository
}

func hasGitMetadata(root string) (bool, error) {
	for directory := filepath.Clean(root); ; directory = filepath.Dir(directory) {
		_, err := os.Lstat(filepath.Join(directory, ".git"))
		switch {
		case err == nil:
			return true, nil
		case !errors.Is(err, os.ErrNotExist):
			return false, err
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return false, nil
		}
	}
}

func (r *Repository) run(ctx context.Context, root string, args ...string) ([]byte, gitmodel.State) {
	result := r.runProcess(ctx, root, args...)
	if result.state != gitmodel.StateAvailable {
		return nil, result.state
	}
	if result.err != nil {
		return nil, gitmodel.StateUnavailable
	}
	return result.output, gitmodel.StateAvailable
}

type processResult struct {
	output []byte
	state  gitmodel.State
	err    error
}

func (r *Repository) runProcess(ctx context.Context, root string, args ...string) processResult {
	executable, err := r.lookPath(r.config.Executable)
	if err != nil {
		return processResult{state: gitmodel.StateExecutableUnavailable}
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = root
	baseEnvironment := os.Environ()
	if r.environ != nil {
		baseEnvironment = r.environ()
	}
	cmd.Env = gitEnvironment(baseEnvironment)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdoutCapture := boundedCapture{Writer: &stdout, Limit: r.config.MaxStdoutBytes}
	stderrCapture := boundedCapture{Writer: &stderr, Limit: r.config.MaxStderrBytes}
	cmd.Stdout = &stdoutCapture
	cmd.Stderr = &stderrCapture
	err = cmd.Run()
	if ctx.Err() != nil {
		return processResult{state: gitmodel.StateUnavailable}
	}
	if stdoutCapture.Overflow || stderrCapture.Overflow {
		return processResult{state: gitmodel.StateTooLarge}
	}
	return processResult{output: stdout.Bytes(), state: gitmodel.StateAvailable, err: err}
}

func gitEnvironment(base []string) []string {
	environment := make([]string, 0, len(base)+2)
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if found && strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat")
}

type boundedCapture struct {
	Writer   *bytes.Buffer
	Limit    int64
	Written  int64
	Overflow bool
}

func (w *boundedCapture) Write(data []byte) (int, error) {
	remaining := w.Limit - w.Written
	if remaining <= 0 {
		w.Overflow = true
		return len(data), nil
	}
	captured := len(data)
	if int64(captured) > remaining {
		captured = int(remaining)
		w.Overflow = true
	}
	_, _ = w.Writer.Write(data[:captured])
	w.Written += int64(captured)
	return len(data), nil
}

func parseStatus(data []byte) ([]gitmodel.Change, error) {
	var changes []gitmodel.Change
	for len(data) > 0 {
		end := bytes.IndexByte(data, 0)
		if end < 0 {
			return nil, errors.New("unterminated Git status record")
		}
		record := string(data[:end])
		data = data[end+1:]
		if len(record) < 4 {
			return nil, errors.New("invalid Git status record")
		}
		change := gitmodel.Change{IndexStatus: record[:1], WorktreeStatus: record[1:2], Path: record[3:]}
		if change.IndexStatus == "?" && change.WorktreeStatus == "?" {
			change.Untracked = true
			change.AvailableLayers = []gitmodel.Layer{gitmodel.LayerUntracked}
		} else if change.IndexStatus == "U" || change.WorktreeStatus == "U" || change.IndexStatus+change.WorktreeStatus == "AA" || change.IndexStatus+change.WorktreeStatus == "DD" {
			change.Unmerged = true
		} else {
			if change.IndexStatus != " " {
				change.AvailableLayers = append(change.AvailableLayers, gitmodel.LayerStaged)
			}
			if change.WorktreeStatus != " " {
				change.AvailableLayers = append(change.AvailableLayers, gitmodel.LayerUnstaged)
			}
		}
		if isRenameOrCopy(change) {
			end = bytes.IndexByte(data, 0)
			if end < 0 {
				return nil, errors.New("missing Git rename or copy source")
			}
			change.OriginalPath, data = string(data[:end]), data[end+1:]
		}
		if err := validateGitPath(change.Path); err != nil {
			return nil, err
		}
		if change.OriginalPath != "" {
			if err := validateGitPath(change.OriginalPath); err != nil {
				return nil, err
			}
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func isRenameOrCopy(change gitmodel.Change) bool {
	return change.IndexStatus == "R" ||
		change.IndexStatus == "C" ||
		change.WorktreeStatus == "R" ||
		change.WorktreeStatus == "C"
}

// workspacePathArgument verifies the workspace-to-repository relationship before
// returning the workspace-relative logical path. Git commands use the registered
// workspace as their working directory, so that logical path is the safe pathspec.
func (r *Repository) workspacePathArgument(ctx context.Context, root string, path string) (string, gitmodel.State) {
	if _, state := r.workspacePrefix(ctx, root); state != gitmodel.StateAvailable {
		return "", state
	}
	parsed, err := filemodel.ParseRelativePath(path, false)
	if err != nil || parsed.String() != path {
		return "", gitmodel.StateUnavailable
	}
	return parsed.String(), gitmodel.StateAvailable
}

func (r *Repository) workspacePrefix(ctx context.Context, root string) (string, gitmodel.State) {
	topLevel, state := r.run(ctx, root, "rev-parse", "--show-toplevel")
	if state != gitmodel.StateAvailable {
		return "", state
	}
	prefix, err := workspacePrefix(root, strings.TrimSpace(string(topLevel)))
	if err != nil {
		return "", gitmodel.StateUnavailable
	}
	return prefix, gitmodel.StateAvailable
}

func workspacePrefix(root string, repositoryRoot string) (string, error) {
	relative, err := filepath.Rel(repositoryRoot, root)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("workspace is outside Git repository")
	}
	if relative == "." {
		return "", nil
	}
	return filepath.ToSlash(relative), nil
}

func restrictToWorkspace(changes []gitmodel.Change, prefix string) []gitmodel.Change {
	result := make([]gitmodel.Change, 0, len(changes))
	for _, change := range changes {
		path, ok := workspaceRelativePath(change.Path, prefix)
		if !ok {
			continue
		}
		change.Path = path
		if change.OriginalPath != "" {
			originalPath, ok := workspaceRelativePath(change.OriginalPath, prefix)
			if !ok {
				continue
			}
			change.OriginalPath = originalPath
		}
		result = append(result, change)
	}
	return result
}

func workspaceRelativePath(value string, prefix string) (string, bool) {
	if prefix == "" {
		return value, true
	}
	prefix += "/"
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	return strings.TrimPrefix(value, prefix), true
}

func repositoryPath(prefix string, value string) string {
	if prefix == "" {
		return value
	}
	return prefix + "/" + value
}

func findChange(changes []gitmodel.Change, path string) (gitmodel.Change, bool) {
	for _, change := range changes {
		if change.Path == path {
			return change, true
		}
	}
	return gitmodel.Change{}, false
}
func hasLayer(change gitmodel.Change, layer gitmodel.Layer) bool {
	return slices.Contains(change.AvailableLayers, layer)
}
func validateGitPath(value string) error {
	parsed, err := filemodel.ParseRelativePath(value, false)
	if err != nil || parsed.String() != value {
		return errors.New("invalid Git path")
	}
	return nil
}
func (r *Repository) readWorktree(ctx context.Context, root string, path string) ([]byte, gitmodel.State) {
	if r.files == nil {
		return nil, gitmodel.StateUnavailable
	}
	parsed, err := filemodel.ParseRelativePath(path, false)
	if err != nil {
		return nil, gitmodel.StateUnavailable
	}
	result, err := r.files.Read(ctx, root, parsed)
	if err != nil {
		switch filemodel.CodeOf(err) {
		case "file_too_large":
			return nil, gitmodel.StateTooLarge
		case "file_not_text", "symlink_unsupported":
			return nil, gitmodel.StateBinary
		case "type_mismatch":
			return nil, gitmodel.StateSubmodule
		default:
			return nil, gitmodel.StateUnavailable
		}
	}
	return []byte(result.Text), gitmodel.StateAvailable
}

func (r *Repository) exceedsTextLimit(data []byte) bool {
	return int64(len(data)) > r.config.MaxTextBytes
}

func text(data []byte) bool { return utf8.Valid(data) && !bytes.Contains(data, []byte{0}) }

var _ gitmodel.Repository = (*Repository)(nil)
