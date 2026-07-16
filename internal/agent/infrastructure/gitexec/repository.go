package gitexec

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

type worktreeReader interface {
	Read(ctx context.Context, root string, path filemodel.RelativePath) (filemodel.ReadResult, error)
}

type Repository struct {
	config   Config
	files    worktreeReader
	lookPath func(string) (string, error)
}

func New(config Config, files *workspacefile.Store) *Repository {
	return &Repository{config: config, files: files, lookPath: exec.LookPath}
}

func (r *Repository) Status(ctx context.Context, root string) (gitmodel.StatusResult, error) {
	inside, state := r.run(ctx, root, "rev-parse", "--is-inside-work-tree")
	if state != gitmodel.StateAvailable {
		return gitmodel.StatusResult{State: state}, nil
	}
	if strings.TrimSpace(string(inside)) != "true" {
		return gitmodel.StatusResult{State: gitmodel.StateNotRepository}, nil
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
	if !ok || !hasLayer(change, request.Layer) {
		return gitmodel.DiffResult{State: gitmodel.StateLayerUnavailable}, nil
	}
	if change.Unmerged {
		return gitmodel.DiffResult{State: gitmodel.StateUnmerged}, nil
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

func (r *Repository) diff(ctx context.Context, root string, change gitmodel.Change, layer gitmodel.Layer, prefix string) (gitmodel.DiffResult, error) {
	contentPath := change.OriginalPath
	if contentPath == "" {
		contentPath = change.Path
	}
	repositoryContentPath := repositoryPath(prefix, contentPath)
	repositoryModifiedPath := repositoryPath(prefix, change.Path)
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
			original, state = r.show(ctx, root, ":"+repositoryModifiedPath)
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

func (r *Repository) show(ctx context.Context, root string, object string) ([]byte, gitmodel.State) {
	output, state := r.run(ctx, root, "show", "--no-ext-diff", "--no-textconv", object, "--")
	return output, state
}

func (r *Repository) run(ctx context.Context, root string, args ...string) ([]byte, gitmodel.State) {
	executable, err := r.lookPath(r.config.Executable)
	if err != nil {
		return nil, gitmodel.StateExecutableUnavailable
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdoutCapture := boundedCapture{Writer: &stdout, Limit: r.config.MaxStdoutBytes}
	stderrCapture := boundedCapture{Writer: &stderr, Limit: r.config.MaxStderrBytes}
	cmd.Stdout = &stdoutCapture
	cmd.Stderr = &stderrCapture
	err = cmd.Run()
	if ctx.Err() != nil {
		return nil, gitmodel.StateUnavailable
	}
	if stdoutCapture.Overflow || stderrCapture.Overflow {
		return nil, gitmodel.StateTooLarge
	}
	if err != nil {
		return nil, gitmodel.StateUnavailable
	}
	return stdout.Bytes(), gitmodel.StateAvailable
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
		if change.IndexStatus == "R" || change.IndexStatus == "C" {
			end = bytes.IndexByte(data, 0)
			if end < 0 {
				return nil, errors.New("missing Git rename source")
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
	for _, available := range change.AvailableLayers {
		if available == layer {
			return true
		}
	}
	return false
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
