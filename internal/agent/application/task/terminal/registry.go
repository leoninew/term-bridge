package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"log/slog"

	sessionapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/session"
	workspaceapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/workspace"
	termpty "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/pty"
	"gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/storage/history"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/session"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
	"gitee.com/leoninew/TermBridge-go/internal/agent/repository/task/state"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"

	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/prototime"
	terminalproto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/terminal"
)

const (
	AttachmentUnattached  AttachmentState = "unattached"
	AttachmentAttached    AttachmentState = "attached"
	AttachmentDetached    AttachmentState = "detached"
	AttachmentReattaching AttachmentState = "reattaching"

	DefaultClientQueueSize  = 64
	DefaultClientQueueBytes = 4 * 1024 * 1024
	DefaultReplayMaxBytes   = 1024 * 1024
	DefaultReplayChunkBytes = 64 * 1024
)

type AttachmentState string

type RuntimeStore interface {
	FindWorkspaceByPath(path string) (workspace.Workspace, error)
	FindWorkspaceById(workspaceId string) (workspace.Workspace, error)
	SaveWorkspace(workspace.Workspace) error
	LoadWorkspace(workspaceId string) (workspace.Workspace, error)
	SaveSession(session.Session) error
	LoadSession(workspaceId string, sessionId string) (session.Session, error)
	UpdateSession(workspaceId string, sessionId string, update func(*session.Session) error) (session.Session, error)
	UpdateTerminalSession(workspaceId string, sessionId string, update func(*session.Session) error) (session.Session, error)
	DeleteSession(workspaceId string, sessionId string) error
	DeleteWorkspace(workspaceId string) error
	SaveState(workspaceId string, sessionId string, value session.StateRecord) error
	LoadState(workspaceId string, sessionId string) (session.StateRecord, error)
	SaveProcess(workspaceId string, sessionId string, value process.Record) error
	SaveProcessState(workspaceId string, sessionId string, value process.Record, state session.StateRecord) error
	LoadProcess(workspaceId string, sessionId string) (process.Record, error)
	SaveExit(workspaceId string, sessionId string, value process.ExitRecord) error
	SaveExitState(workspaceId string, sessionId string, value process.ExitRecord, state session.StateRecord) error
	SaveSessionExitState(value session.Session, exit process.ExitRecord, state session.StateRecord) error
	LoadExit(workspaceId string, sessionId string) (process.ExitRecord, error)
	HistoryPath(workspaceId string, sessionId string) string
	ListWorkspaces() ([]workspace.Workspace, []state.Warning, error)
	ListSessionsByWorkspaceId(workspaceId string) ([]session.View, []state.Warning, error)
	ListSessions() ([]session.View, []state.Warning, error)
	UpdateWorkspaceOrder(workspaceIds []string, now time.Time) ([]workspace.Workspace, error)
	UpdateSessionOrder(workspaceId string, sessionIds []string, now time.Time) ([]session.View, []state.Warning, error)
}

type sessionRunStarter interface {
	BeginSessionRun(workspaceId string, sessionId string, size process.TerminalSize) error
}

type Config struct {
	Cwd              string
	Store            RuntimeStore
	LogDir           string
	History          history.Config
	Manager          termpty.Manager
	Logger           *slog.Logger
	ClientQueueSize  int
	ClientQueueBytes int
	ReplayMaxBytes   int64
	ReplayChunkBytes int
}

type Registry struct {
	cwd              string
	store            RuntimeStore
	logDir           string
	historyConfig    history.Config
	manager          termpty.Manager
	logger           *slog.Logger
	clientQueueSize  int
	clientQueueBytes int
	replayMaxBytes   int64
	replayChunkBytes int

	mu        sync.Mutex
	runtimes  map[string]*SessionRuntime
	launching map[string]bool
}

type OutboundKind int

const (
	OutboundText OutboundKind = iota + 1
	OutboundBinary
)

type Outbound struct {
	Kind   OutboundKind
	Text   *agent.ServerControlMessage
	Binary []byte
}

type Client struct {
	id               string
	runtime          *SessionRuntime
	queue            chan Outbound
	once             sync.Once
	mu               sync.Mutex
	queuedBytes      int
	sentBinaryChunks int
	sentBinaryBytes  int64
}

func NewRegistry(config Config) *Registry {
	queueSize := config.ClientQueueSize
	if queueSize <= 0 {
		queueSize = DefaultClientQueueSize
	}
	queueBytes := config.ClientQueueBytes
	if queueBytes <= 0 {
		queueBytes = DefaultClientQueueBytes
	}
	replayMaxBytes := config.ReplayMaxBytes
	if replayMaxBytes <= 0 {
		replayMaxBytes = DefaultReplayMaxBytes
	}
	replayChunkBytes := config.ReplayChunkBytes
	if replayChunkBytes <= 0 {
		replayChunkBytes = DefaultReplayChunkBytes
	}
	return &Registry{
		cwd:              config.Cwd,
		store:            config.Store,
		logDir:           config.LogDir,
		historyConfig:    config.History,
		manager:          config.Manager,
		logger:           config.Logger,
		clientQueueSize:  queueSize,
		clientQueueBytes: queueBytes,
		replayMaxBytes:   replayMaxBytes,
		replayChunkBytes: replayChunkBytes,
		runtimes:         map[string]*SessionRuntime{},
		launching:        map[string]bool{},
	}
}

func (r *Registry) workspaceForCreateSession(workspaceId string, cwd string) (workspace.Workspace, error) {
	if strings.TrimSpace(workspaceId) != "" {
		ws, err := r.store.LoadWorkspace(workspaceId)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return workspace.Workspace{}, apperrors.NotFound("workspace not found", err)
			}
			return workspace.Workspace{}, apperrors.Runtime("load workspace", err)
		}
		return ws, nil
	}
	resolver := workspaceapp.Resolver{Store: r.store}
	ws, err := resolver.Resolve(cwd)
	if err != nil {
		return workspace.Workspace{}, apperrors.Runtime("resolve workspace", err)
	}
	return ws, nil
}

func (r *Registry) CreateSession(ctx context.Context, request *agent.CreateSessionReq) (*agent.CreateSessionResp, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, apperrors.Usage("missing session name")
	}
	command := request.GetCommand()
	if len(command) != 1 {
		return nil, apperrors.Usage("session command must contain one raw command text")
	}
	if strings.TrimSpace(command[0]) == "" {
		return nil, apperrors.Usage("missing session command")
	}
	commandText := command[0]
	cwd := request.GetCwd()
	if strings.TrimSpace(cwd) == "" {
		cwd = r.cwd
	}
	absCwd, err := resolveSessionCwd(cwd)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(absCwd); err != nil {
		return nil, apperrors.Config("invalid session cwd", err)
	} else if !info.IsDir() {
		return nil, apperrors.Config("invalid session cwd", fmt.Errorf("not a directory"))
	}
	size := process.TerminalSize{Cols: int(request.GetCols()), Rows: int(request.GetRows())}.OrDefault()
	if err := terminalproto.ValidateSize(size.Cols, size.Rows); err != nil {
		return nil, apperrors.Usage(err.Error())
	}
	r.logger.Info("terminal session create request", "name", name, "cwd", absCwd, "command_length", len(commandText), "cols", size.Cols, "rows", size.Rows)

	ws, err := r.workspaceForCreateSession(request.GetWorkspaceId(), absCwd)
	if err != nil {
		r.logger.Error("terminal session workspace resolve failed", "source", "web", "cwd", absCwd, "stage", "resolve_workspace", "error_kind", apperrors.KindOf(err))
		return nil, err
	}

	env := os.Environ()
	commandRecord := session.CommandRecord{
		Command:              commandText,
		EnvStrategy:          "inherit",
		EnvCount:             len(env),
		Source:               session.CommandSource(request.GetCommandSource()),
		ShortcutIdSnapshot:   request.GetShortcutIdSnapshot(),
		ShortcutNameSnapshot: request.GetShortcutNameSnapshot(),
	}
	if !commandRecord.ValidSource() {
		return nil, apperrors.Usage("invalid session command source")
	}
	manager := sessionapp.Manager{Store: r.store}
	sess, err := manager.Create(sessionapp.CreateOptions{
		Workspace: ws,
		Name:      name,
		LaunchCwd: absCwd,
		Command:   commandRecord,
		History: session.HistoryRecord{
			Path:         "history.log",
			MaxLines:     r.historyConfig.MaxLines,
			MaxBytes:     r.historyConfig.MaxBytes,
			MaxLineBytes: r.historyConfig.MaxLineBytes,
		},
	})
	if err != nil {
		r.logger.Error("terminal session create failed", "source", "web", "workspace_id", ws.Id, "cwd", absCwd, "stage", "create_session", "error_kind", apperrors.KindOf(err))
		return nil, apperrors.Runtime("create session", err)
	}
	r.logger.Info("terminal session created", "source", "web", "session_id", sess.Id, "workspace_id", sess.WorkspaceId, "cwd", absCwd, "command_length", len(commandText))

	if err := r.startSessionRuntime(ctx, sess, commandText, size); err != nil {
		reason := startFailureReason(err)
		r.saveSessionFailed(sess, reason)
		r.logger.Error("terminal session start failed", "source", "web", "session_id", sess.Id, "workspace_id", sess.WorkspaceId, "cwd", sess.LaunchCwd, "stage", "start_process", "reason", reason, "error_kind", apperrors.KindOf(err))
		return nil, err
	}
	return &agent.CreateSessionResp{SessionId: sess.Id, WorkspaceId: sess.WorkspaceId, State: string(session.StateRunning)}, nil
}

func (r *Registry) RerunSession(ctx context.Context, workspaceId string, sessionId string, request *agent.RerunSessionReq) (*agent.CreateSessionResp, error) {
	view, err := r.sessionView(workspaceId, sessionId)
	if err != nil {
		return nil, err
	}
	if !session.Terminal(view.State.State) {
		return nil, apperrors.Usage("cannot rerun running session")
	}
	commandText := commandTextFromSession(view.Session)
	if strings.TrimSpace(commandText) == "" {
		r.saveSessionFailed(view.Session, "missing_rerun_command")
		return nil, apperrors.Usage("missing session command")
	}
	if err := validateSessionCwd(view.Session.LaunchCwd); err != nil {
		r.saveSessionFailed(view.Session, "invalid_rerun_cwd")
		return nil, err
	}
	size := process.TerminalSize{Cols: int(request.GetCols()), Rows: int(request.GetRows())}.OrDefault()
	if err := terminalproto.ValidateSize(size.Cols, size.Rows); err != nil {
		return nil, apperrors.Usage(err.Error())
	}
	if err := r.claimSessionStart(sessionId); err != nil {
		return nil, err
	}
	defer r.releaseSessionStart(sessionId)

	archiveId := archiveIdForTime(time.Now().UTC())
	_, err = r.archiveCurrentHistory(workspaceId, sessionId, archiveId)
	if err != nil {
		r.saveSessionFailed(view.Session, "archive_history_failed")
		return nil, apperrors.Runtime("archive history", err)
	}
	if starter, ok := r.store.(sessionRunStarter); ok {
		if err := starter.BeginSessionRun(workspaceId, sessionId, size); err != nil {
			r.saveSessionFailed(view.Session, "begin_session_run_failed")
			return nil, apperrors.Runtime("begin session run", err)
		}
	}
	if err := r.startClaimedSessionRuntime(ctx, view.Session, commandText, size); err != nil {
		reason := startFailureReason(err)
		r.saveSessionFailed(view.Session, reason)
		r.logger.Error("terminal session rerun failed", "source", "web", "session_id", view.Session.Id, "workspace_id", view.Session.WorkspaceId, "cwd", view.Session.LaunchCwd, "stage", "rerun_process", "reason", reason, "error_kind", apperrors.KindOf(err))
		return nil, err
	}
	return &agent.CreateSessionResp{SessionId: view.Session.Id, WorkspaceId: view.Session.WorkspaceId, State: string(session.StateRunning)}, nil
}

func (r *Registry) startSessionRuntime(ctx context.Context, sess session.Session, commandText string, size process.TerminalSize) error {
	if err := r.claimSessionStart(sess.Id); err != nil {
		return err
	}
	defer r.releaseSessionStart(sess.Id)
	return r.startClaimedSessionRuntime(ctx, sess, commandText, size)
}

func (r *Registry) startClaimedSessionRuntime(ctx context.Context, sess session.Session, commandText string, size process.TerminalSize) error {
	historyWriter, err := history.NewWriter(r.store.HistoryPath(sess.WorkspaceId, sess.Id), history.Config{MaxLines: r.historyConfig.MaxLines, MaxBytes: r.historyConfig.MaxBytes, MaxLineBytes: r.historyConfig.MaxLineBytes})
	if err != nil {
		return apperrors.Runtime("create history writer", err)
	}
	spec, err := process.NewSpec(sess.LaunchCwd, commandText, size)
	if err != nil {
		_ = historyWriter.Close()
		return apperrors.Runtime("build process spec", err)
	}
	spec.Env = os.Environ()

	r.logger.Info("terminal process start requested", "source", "web", "session_id", sess.Id, "workspace_id", sess.WorkspaceId, "cwd", spec.Cwd, "cols", spec.InitialSize.Cols, "rows", spec.InitialSize.Rows, "command_length", len(spec.CommandText))
	ptySession, err := r.manager.Start(ctx, spec)
	if err != nil {
		_ = historyWriter.Close()
		r.logger.Error("terminal process start failed", "source", "web", "session_id", sess.Id, "workspace_id", sess.WorkspaceId, "cwd", spec.Cwd, "stage", "start_pty", "error_kind", apperrors.KindOf(err))
		if apperrors.IsUsage(err) {
			return err
		}
		return apperrors.Runtime("start pty", err)
	}
	if reporter, ok := ptySession.(termpty.ProcessReporter); ok {
		record := reporter.ProcessInfo()
		if record.CommandLine == "" {
			record.CommandLine = spec.CommandText
		}
		if record.Cwd == "" {
			record.Cwd = spec.Cwd
		}
		if record.StartedAt.IsZero() {
			record.StartedAt = time.Now().UTC()
		}
		stateRecord := session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: record.StartedAt}
		if err := r.store.SaveProcessState(sess.WorkspaceId, sess.Id, record, stateRecord); err != nil {
			_ = ptySession.Close()
			_ = historyWriter.Close()
			return apperrors.Runtime("save process state", err)
		}
	} else if err := r.store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: time.Now().UTC()}); err != nil {
		_ = ptySession.Close()
		_ = historyWriter.Close()
		return apperrors.Runtime("save running state", err)
	}

	r.logger.Info("terminal process started", "source", "web", "session_id", sess.Id, "workspace_id", sess.WorkspaceId, "cwd", spec.Cwd)
	runtime := newSessionRuntime(r, sess, ptySession, historyWriter, size)
	r.mu.Lock()
	r.runtimes[sess.Id] = runtime
	r.mu.Unlock()
	runtime.start()
	return nil
}

func (r *Registry) claimSessionStart(sessionId string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runtimes[sessionId] != nil || r.launching[sessionId] {
		return apperrors.Usage("session already running")
	}
	r.launching[sessionId] = true
	return nil
}

func (r *Registry) releaseSessionStart(sessionId string) {
	r.mu.Lock()
	delete(r.launching, sessionId)
	r.mu.Unlock()
}

func (r *Registry) saveSessionFailed(sess session.Session, reason string) {
	if err := r.store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: reason, UpdatedAt: time.Now().UTC()}); err != nil {
		r.logger.Warn("save failed session state", "session_id", sess.Id, "reason", reason, "error_kind", apperrors.KindOf(err))
	}
}

func startFailureReason(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "create history writer"):
		return "history_create_failed"
	case strings.Contains(message, "build process spec"):
		return "build_process_spec_failed"
	case strings.Contains(message, "start pty"):
		return "pty_start_failed"
	case strings.Contains(message, "save process"):
		return "save_process_failed"
	case strings.Contains(message, "save running state"):
		return "save_running_state_failed"
	default:
		return "runtime_start_failed"
	}
}

func commandTextFromSession(sess session.Session) string {
	return sess.Command.Command
}

func validateSessionCwd(cwd string) error {
	if strings.TrimSpace(cwd) == "" {
		return apperrors.Config("invalid session cwd", fmt.Errorf("missing cwd"))
	}
	if info, err := os.Stat(cwd); err != nil {
		return apperrors.Config("invalid session cwd", err)
	} else if !info.IsDir() {
		return apperrors.Config("invalid session cwd", fmt.Errorf("not a directory"))
	}
	return nil
}

func (r *Registry) archiveCurrentHistory(workspaceId string, sessionId string, archiveId string) (string, error) {
	historyPath := r.store.HistoryPath(workspaceId, sessionId)
	if _, err := os.Stat(historyPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	for i := range 1000 {
		archiveName := "history." + archiveId + archiveSuffix(i) + ".log"
		archivePath := filepath.Join(filepath.Dir(historyPath), archiveName)
		if _, err := os.Stat(archivePath); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
			return "", err
		}
		if err := os.Rename(historyPath, archivePath); err != nil {
			return "", err
		}
		return archiveName, nil
	}
	return "", fmt.Errorf("history archive name exhausted for %s", archiveId)
}

func archiveSuffix(index int) string {
	if index == 0 {
		return ""
	}
	return fmt.Sprintf(".%d", index)
}

func archiveIdForTime(value time.Time) string {
	return value.UTC().Format("20060102T150405.000000000Z")
}

func (r *Registry) Attach(workspaceId string, sessionId string) (*Client, error) {
	r.mu.Lock()
	runtime := r.runtimes[sessionId]
	r.mu.Unlock()
	if runtime == nil || runtime.session.WorkspaceId != workspaceId {
		return nil, apperrors.Runtime("session not attachable", fmt.Errorf("live PTY handle not found for %s/%s", workspaceId, sessionId))
	}
	return runtime.attach()
}

func (r *Registry) CloseSession(workspaceId string, sessionId string, reason string) (*agent.SessionSummary, error) {
	r.mu.Lock()
	runtime := r.runtimes[sessionId]
	r.mu.Unlock()
	if runtime == nil || runtime.session.WorkspaceId != workspaceId {
		return r.closeMissingRuntimeSession(workspaceId, sessionId, reason)
	}
	if err := runtime.closeSession(reason); err != nil {
		return nil, err
	}
	return r.GetSession(workspaceId, sessionId)
}

func (r *Registry) closeMissingRuntimeSession(workspaceId string, sessionId string, reason string) (*agent.SessionSummary, error) {
	view, err := r.sessionView(workspaceId, sessionId)
	if err != nil {
		return nil, err
	}
	r.logger.Warn("close session missing live PTY handle", "workspace_id", workspaceId, "session_id", sessionId, "reason", reason)
	if !session.Terminal(view.State.State) {
		now := time.Now().UTC()
		stateRecord := session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "missing_pty_on_close", UpdatedAt: now}
		if err := r.store.SaveState(workspaceId, sessionId, stateRecord); err != nil {
			return nil, apperrors.Runtime("save missing PTY close state", err)
		}
		view.State = stateRecord
	}
	return r.summaryFromView(view), nil
}

func (r *Registry) ListWorkspaces() ([]*agent.Workspace, error) {
	workspaces, _, err := r.store.ListWorkspaces()
	if err != nil {
		return nil, apperrors.Runtime("list workspaces", err)
	}
	out := make([]*agent.Workspace, 0, len(workspaces))
	for _, ws := range workspaces {
		out = append(out, summaryFromWorkspace(ws))
	}
	return out, nil
}

func (r *Registry) WorkspaceTree() ([]*agent.WorkspaceTreeNode, error) {
	workspaces, _, err := r.store.ListWorkspaces()
	if err != nil {
		return nil, apperrors.Runtime("list workspaces", err)
	}
	nodes := make([]*agent.WorkspaceTreeNode, 0, len(workspaces))
	for _, ws := range workspaces {
		views, _, err := r.store.ListSessionsByWorkspaceId(ws.Id)
		if err != nil {
			return nil, apperrors.Runtime("list workspace sessions", err)
		}
		nodes = append(nodes, &agent.WorkspaceTreeNode{
			Id:        ws.Id,
			Name:      ws.Name,
			Path:      ws.Path,
			UpdatedAt: prototime.FromTime(ws.UpdatedAt),
			Children:  r.workspaceSessionSummariesFromViews(views),
		})
	}
	return nodes, nil
}

func (r *Registry) ListSessions() ([]*agent.SessionSummary, error) {
	views, _, err := r.store.ListSessions()
	if err != nil {
		return nil, apperrors.Runtime("list sessions", err)
	}
	return r.summariesFromViews(views), nil
}

func (r *Registry) ListSessionsByWorkspaceId(workspaceId string) ([]*agent.SessionSummary, error) {
	views, _, err := r.store.ListSessionsByWorkspaceId(workspaceId)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperrors.NotFound("workspace not found", err)
		}
		return nil, apperrors.Runtime("list workspace sessions", err)
	}
	return r.workspaceSessionSummariesFromViews(views), nil
}

func (r *Registry) UpdateSessionOrder(workspaceId string, sessionIds []string) ([]*agent.SessionSummary, error) {
	if len(sessionIds) == 0 {
		return nil, apperrors.Usage("session_ids is required")
	}
	views, _, err := r.store.UpdateSessionOrder(workspaceId, sessionIds, time.Now().UTC())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if strings.Contains(err.Error(), "session_id") {
				return nil, apperrors.NotFound("session not found", err)
			}
			return nil, apperrors.NotFound("workspace not found", err)
		}
		if strings.Contains(err.Error(), "duplicate session_id") {
			return nil, apperrors.Usage(err.Error())
		}
		return nil, apperrors.Runtime("update session order", err)
	}
	return r.workspaceSessionSummariesFromViews(views), nil
}

func (r *Registry) UpdateSession(workspaceId string, sessionId string, request *agent.UpdateSessionReq) (*agent.SessionSummary, error) {
	if request == nil || (request.Name == nil && request.Command == nil) {
		return nil, apperrors.Usage("session update requires name or command")
	}
	var name string
	if request.Name != nil {
		name = strings.TrimSpace(request.GetName())
		if name == "" {
			return nil, apperrors.Usage("missing session name")
		}
	}
	commandSourceProvided := request.CommandSource != nil || request.ShortcutIdSnapshot != nil || request.ShortcutNameSnapshot != nil
	if request.Command == nil && commandSourceProvided {
		return nil, apperrors.Usage("session command source requires command")
	}
	var commandText string
	if request.Command != nil {
		commandText = request.GetCommand()
		if strings.TrimSpace(commandText) == "" {
			return nil, apperrors.Usage("missing session command")
		}
	}
	view, err := r.sessionView(workspaceId, sessionId)
	if err != nil {
		return nil, err
	}
	if !session.Terminal(view.State.State) {
		return nil, apperrors.Usage("only stopped or failed sessions can be edited")
	}
	updated, err := r.store.UpdateTerminalSession(workspaceId, sessionId, func(value *session.Session) error {
		if request.Name != nil {
			value.Name = name
		}
		if request.Command != nil {
			if commandSourceProvided {
				value.Command.Source = session.CommandSource(request.GetCommandSource())
				value.Command.ShortcutIdSnapshot = request.GetShortcutIdSnapshot()
				value.Command.ShortcutNameSnapshot = request.GetShortcutNameSnapshot()
				if !value.Command.ValidSource() {
					return apperrors.Usage("invalid session command source")
				}
			}
			value.Command.Command = commandText
		}
		value.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "no longer terminal") {
			return nil, apperrors.Usage("only stopped or failed sessions can be edited")
		}
		return nil, apperrors.Runtime("update session", err)
	}
	view.Session = updated
	view.CommandText = commandTextFromSession(updated)
	return r.summaryFromView(view), nil
}

func (r *Registry) DeleteSession(workspaceId string, sessionId string) error {
	view, err := r.sessionView(workspaceId, sessionId)
	if err != nil {
		return err
	}
	if r.hasRuntime(sessionId) || !session.Terminal(view.State.State) {
		return apperrors.Usage("cannot delete running session")
	}
	if err := r.store.DeleteSession(workspaceId, sessionId); err != nil {
		return apperrors.Runtime("delete session", err)
	}
	return nil
}

func (r *Registry) UpdateWorkspaceOrder(workspaceIds []string) ([]*agent.Workspace, error) {
	if len(workspaceIds) == 0 {
		return nil, apperrors.Usage("workspace_ids is required")
	}
	updated, err := r.store.UpdateWorkspaceOrder(workspaceIds, time.Now().UTC())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperrors.NotFound("workspace not found", err)
		}
		if strings.Contains(err.Error(), "duplicate workspace_id") {
			return nil, apperrors.Usage(err.Error())
		}
		return nil, apperrors.Runtime("update workspace order", err)
	}
	out := make([]*agent.Workspace, 0, len(updated))
	for _, ws := range updated {
		out = append(out, summaryFromWorkspace(ws))
	}
	return out, nil
}

func (r *Registry) DeleteWorkspace(workspaceId string) error {
	ws, err := r.store.FindWorkspaceById(workspaceId)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return apperrors.NotFound("workspace not found", err)
		}
		return apperrors.Runtime("load workspace", err)
	}
	views, _, err := r.store.ListSessionsByWorkspaceId(workspaceId)
	if err != nil {
		return apperrors.Runtime("list workspace sessions", err)
	}
	for _, view := range views {
		if r.hasRuntime(view.Session.Id) || !session.Terminal(view.State.State) {
			return apperrors.Usage("cannot delete workspace with running sessions")
		}
	}
	if err := r.store.DeleteWorkspace(ws.Id); err != nil {
		return apperrors.Runtime("delete workspace", err)
	}
	return nil
}

func (r *Registry) GetSession(workspaceId string, sessionId string) (*agent.SessionSummary, error) {
	view, err := r.sessionView(workspaceId, sessionId)
	if err != nil {
		return nil, err
	}
	return r.summaryFromView(view), nil
}

func (r *Registry) History(workspaceId string, sessionId string) ([]byte, error) {
	r.mu.Lock()
	runtime := r.runtimes[sessionId]
	r.mu.Unlock()
	if runtime != nil {
		if runtime.session.WorkspaceId != workspaceId {
			return nil, apperrors.NotFound("session not found", os.ErrNotExist)
		}
		if err := runtime.history.Flush(); err != nil {
			return nil, apperrors.Runtime("flush history", err)
		}
	}
	if _, err := r.sessionView(workspaceId, sessionId); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(r.store.HistoryPath(workspaceId, sessionId))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, apperrors.Runtime("read history", err)
	}
	return data, nil
}

func resolveSessionCwd(path string) (string, error) {
	path, err := expandHome(path)
	if err != nil {
		return "", apperrors.Config("resolve session cwd", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", apperrors.Config("resolve session cwd", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", apperrors.Config("resolve session cwd", err)
	}
	return filepath.Clean(resolved), nil
}

func expandHome(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	if strings.HasPrefix(path, "~") {
		return "", fmt.Errorf("unsupported home path %q", path)
	}
	return path, nil
}

func (r *Registry) removeRuntime(sessionId string) {
	r.mu.Lock()
	delete(r.runtimes, sessionId)
	r.mu.Unlock()
}

func (r *Registry) hasRuntime(sessionId string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runtimes[sessionId] != nil
}

func (r *Registry) sessionView(workspaceId string, sessionId string) (session.View, error) {
	views, _, err := r.store.ListSessionsByWorkspaceId(workspaceId)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return session.View{}, apperrors.NotFound("workspace not found", err)
		}
		return session.View{}, apperrors.Runtime("list workspace sessions", err)
	}
	for _, view := range views {
		if view.Session.Id == sessionId {
			return view, nil
		}
	}
	return session.View{}, apperrors.NotFound("session not found", nil)
}

func (r *Registry) summariesFromViews(views []session.View) []*agent.SessionSummary {
	summaries := r.sessionSummariesFromViews(views)
	sort.Slice(summaries, func(i, j int) bool {
		return prototime.ToTime(summaries[i].GetUpdatedAt()).After(prototime.ToTime(summaries[j].GetUpdatedAt()))
	})
	return summaries
}

func (r *Registry) sessionSummariesFromViews(views []session.View) []*agent.SessionSummary {
	recoverer := session.Recoverer{Store: r.store}
	out := make([]*agent.SessionSummary, 0, len(views))
	for _, view := range views {
		if !r.hasRuntime(view.Session.Id) {
			refreshed, err := recoverer.Refresh(view)
			if err == nil {
				view = refreshed
			}
		}
		out = append(out, r.summaryFromView(view))
	}
	return out
}

func (r *Registry) workspaceSessionSummariesFromViews(views []session.View) []*agent.SessionSummary {
	return r.sessionSummariesFromViews(views)
}

func summaryFromWorkspace(ws workspace.Workspace) *agent.Workspace {
	return &agent.Workspace{Id: ws.Id, Name: ws.Name, Path: ws.Path, UpdatedAt: prototime.FromTime(ws.UpdatedAt)}
}

func (r *Registry) summaryFromView(view session.View) *agent.SessionSummary {
	attachment := AttachmentUnattached
	lifecycleState := view.State.State
	r.mu.Lock()
	if runtime := r.runtimes[view.Session.Id]; runtime != nil {
		attachment = runtime.attachmentState()
		lifecycleState = runtime.lifecycleState()
	}
	r.mu.Unlock()
	return &agent.SessionSummary{
		Id:                   view.Session.Id,
		Name:                 view.Session.Name,
		WorkspaceId:          view.Session.WorkspaceId,
		Command:              view.CommandText,
		Cwd:                  view.Session.LaunchCwd,
		LifecycleState:       string(lifecycleState),
		AttachmentState:      string(attachment),
		ExitCode:             exitCodeFromInt(view.ExitCode),
		UpdatedAt:            prototime.FromTime(view.Session.UpdatedAt),
		CommandSource:        string(view.Session.Command.Source),
		ShortcutIdSnapshot:   view.Session.Command.ShortcutIdSnapshot,
		ShortcutNameSnapshot: view.Session.Command.ShortcutNameSnapshot,
	}
}

func exitCodeFromInt(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func (c *Client) Outbound() <-chan Outbound {
	return c.queue
}

func (c *Client) WriteInput(data []byte) error {
	return c.runtime.writeInput(data)
}

func (c *Client) Resize(cols int, rows int) error {
	return c.runtime.resize(cols, rows)
}

func (c *Client) Detach(reason string) {
	c.once.Do(func() { c.runtime.detachClient(c.id, reason) })
}

func (c *Client) CloseSession() error {
	return c.runtime.closeSession("client_close")
}

func (c *Client) SendControl(message *agent.ServerControlMessage) bool {
	return c.enqueue(Outbound{Kind: OutboundText, Text: message})
}

func (c *Client) enqueue(outbound Outbound) bool {
	size := outboundSize(outbound)
	c.mu.Lock()
	if c.queuedBytes+size > c.runtime.registry.clientQueueBytes {
		c.mu.Unlock()
		return false
	}
	c.queuedBytes += size
	c.mu.Unlock()
	select {
	case c.queue <- outbound:
		return true
	default:
		c.markSent(outbound)
		return false
	}
}

func (c *Client) canEnqueue(size int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.queuedBytes+size <= c.runtime.registry.clientQueueBytes && len(c.queue) < cap(c.queue)
}

func (c *Client) MarkSent(outbound Outbound) {
	c.markSent(outbound)
}

func (c *Client) Id() string {
	return c.id
}

func (c *Client) SessionId() string {
	return c.runtime.session.Id
}

func (c *Client) QueuedBytes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.queuedBytes
}

func (c *Client) MarkBinarySent(bytes int) (chunks int, totalBytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sentBinaryChunks++
	c.sentBinaryBytes += int64(bytes)
	return c.sentBinaryChunks, c.sentBinaryBytes
}

func (c *Client) markSent(outbound Outbound) {
	size := outboundSize(outbound)
	c.mu.Lock()
	c.queuedBytes -= size
	if c.queuedBytes < 0 {
		c.queuedBytes = 0
	}
	c.mu.Unlock()
}

func outboundSize(outbound Outbound) int {
	if outbound.Kind == OutboundBinary {
		return len(outbound.Binary)
	}
	return 512 + len(outbound.Text.Message)
}

func (c *Client) closeQueue() {
	close(c.queue)
}

func copyBytes(data []byte) []byte {
	out := make([]byte, len(data))
	copy(out, data)
	return out
}

func isClosedReadError(err error) bool {
	return errors.Is(err, io.EOF) || strings.Contains(strings.ToLower(err.Error()), "closed")
}
