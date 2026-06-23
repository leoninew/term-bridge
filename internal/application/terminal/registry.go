package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"termbridge-go/internal/domain/identity"
	"termbridge-go/internal/domain/process"
	"termbridge-go/internal/domain/session"
	"termbridge-go/internal/domain/workspace"
	"termbridge-go/internal/infrastructure/config"
	apperrors "termbridge-go/internal/infrastructure/errors"
	"termbridge-go/internal/infrastructure/history"
	"termbridge-go/internal/infrastructure/logging"
	termpty "termbridge-go/internal/infrastructure/pty"
	"termbridge-go/internal/infrastructure/repository/state"
	"termbridge-go/internal/protocol/terminal"
)

const (
	AttachmentUnattached  AttachmentState = "unattached"
	AttachmentAttached    AttachmentState = "attached"
	AttachmentDetached    AttachmentState = "detached"
	AttachmentReattaching AttachmentState = "reattaching"

	DefaultClientQueueSize  = 64
	DefaultClientQueueBytes = 4 * 1024 * 1024
)

type AttachmentState string

type Config struct {
	Cwd              string
	Store            state.Store
	LogDir           string
	History          config.HistoryConfig
	Manager          termpty.Manager
	Logger           *logging.Logger
	ClientQueueSize  int
	ClientQueueBytes int
	EnvDenylist      []string
}

type Registry struct {
	cwd              string
	store            state.Store
	logDir           string
	historyConfig    config.HistoryConfig
	manager          termpty.Manager
	logger           *logging.Logger
	ids              identity.Generator
	clientQueueSize  int
	clientQueueBytes int
	envDenylist      []string

	mu       sync.Mutex
	runtimes map[string]*SessionRuntime
}

type CreateSessionRequest struct {
	WorkspaceId string   `json:"workspace_id,omitempty"`
	Name        string   `json:"name"`
	Cwd         string   `json:"cwd"`
	Command     []string `json:"command"`
	Cols        int      `json:"cols"`
	Rows        int      `json:"rows"`
}

type UpdateSessionRequest struct {
	Name string `json:"name"`
}

func (r *UpdateSessionRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for key := range raw {
		if key != "name" {
			return fmt.Errorf("unsupported session edit field %q", key)
		}
	}
	if value, ok := raw["name"]; ok {
		if err := json.Unmarshal(value, &r.Name); err != nil {
			return err
		}
	}
	return nil
}

type UpdateWorkspaceOrderRequest struct {
	WorkspaceIds []string `json:"workspace_ids"`
}

type WorkspaceTreeNode struct {
	Id        string                    `json:"id"`
	Name      string                    `json:"name"`
	Path      string                    `json:"path"`
	SortOrder int                       `json:"sort_order"`
	UpdatedAt time.Time                 `json:"updated_at"`
	Children  []WorkspaceSessionSummary `json:"children"`
}

type CreateSessionResponse struct {
	SessionId   string `json:"session_id"`
	WorkspaceId string `json:"workspace_id"`
	State       string `json:"state"`
}

type WorkspaceSummary struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	SortOrder int       `json:"sort_order"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionSummary struct {
	Id              string          `json:"id"`
	Name            string          `json:"name"`
	WorkspaceId     string          `json:"workspace_id"`
	Command         string          `json:"command"`
	Cwd             string          `json:"cwd"`
	LifecycleState  session.State   `json:"lifecycle_state"`
	AttachmentState AttachmentState `json:"attachment_state,omitempty"`
	ExitCode        *int            `json:"exit_code,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type WorkspaceSessionSummary struct {
	Id              string          `json:"id"`
	Name            string          `json:"name"`
	Command         string          `json:"command"`
	Cwd             string          `json:"cwd"`
	LifecycleState  session.State   `json:"lifecycle_state"`
	AttachmentState AttachmentState `json:"attachment_state,omitempty"`
	ExitCode        *int            `json:"exit_code,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type OutboundKind int

const (
	OutboundText OutboundKind = iota + 1
	OutboundBinary
)

type Outbound struct {
	Kind   OutboundKind
	Text   terminalproto.ServerMessage
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
	return &Registry{
		cwd:              config.Cwd,
		store:            config.Store,
		logDir:           config.LogDir,
		historyConfig:    config.History,
		manager:          config.Manager,
		logger:           config.Logger,
		ids:              identity.NewUlidGenerator(),
		clientQueueSize:  queueSize,
		clientQueueBytes: queueBytes,
		envDenylist:      append([]string(nil), config.EnvDenylist...),
		runtimes:         map[string]*SessionRuntime{},
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
	resolver := workspace.Resolver{Store: r.store, Ids: r.ids}
	ws, err := resolver.Resolve(cwd)
	if err != nil {
		return workspace.Workspace{}, apperrors.Runtime("resolve workspace", err)
	}
	return ws, nil
}

func (r *Registry) CreateSession(ctx context.Context, request CreateSessionRequest) (CreateSessionResponse, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return CreateSessionResponse{}, apperrors.Usage("missing session name")
	}
	if len(request.Command) == 0 {
		return CreateSessionResponse{}, apperrors.Usage("missing session command")
	}
	cwd := request.Cwd
	if strings.TrimSpace(cwd) == "" {
		cwd = r.cwd
	}
	absCwd, err := resolveSessionCwd(cwd)
	if err != nil {
		return CreateSessionResponse{}, err
	}
	if info, err := os.Stat(absCwd); err != nil {
		return CreateSessionResponse{}, apperrors.Config("invalid session cwd", err)
	} else if !info.IsDir() {
		return CreateSessionResponse{}, apperrors.Config("invalid session cwd", fmt.Errorf("not a directory"))
	}
	size := process.TerminalSize{Cols: request.Cols, Rows: request.Rows}.OrDefault()
	if err := terminalproto.ValidateSize(size.Cols, size.Rows); err != nil {
		return CreateSessionResponse{}, apperrors.Usage(err.Error())
	}
	if r.logger != nil {
		r.logger.Info("terminal session create request", "name", name, "cwd", absCwd, "command", strings.Join(request.Command, " "), "cols", size.Cols, "rows", size.Rows)
	}

	ws, err := r.workspaceForCreateSession(request.WorkspaceId, absCwd)
	if err != nil {
		return CreateSessionResponse{}, err
	}

	env := filterEnv(os.Environ(), r.envDenylist)
	envStrategy := "inherit"
	if len(r.envDenylist) > 0 {
		envStrategy = "inherit_denylist"
	}
	commandRecord := session.CommandRecord{
		Command:     request.Command[0],
		Args:        append([]string(nil), request.Command[1:]...),
		EnvStrategy: envStrategy,
		EnvCount:    len(env),
	}
	manager := session.Manager{Store: r.store, Ids: r.ids}
	sess, err := manager.Create(session.CreateOptions{
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
		return CreateSessionResponse{}, apperrors.Runtime("create session", err)
	}

	historyWriter, err := history.NewWriter(r.store.HistoryPath(sess.WorkspaceId, sess.Id), history.Config{MaxLines: r.historyConfig.MaxLines, MaxBytes: r.historyConfig.MaxBytes, MaxLineBytes: r.historyConfig.MaxLineBytes})
	if err != nil {
		_ = r.store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "history_create_failed", UpdatedAt: time.Now().UTC()})
		return CreateSessionResponse{}, apperrors.Runtime("create history writer", err)
	}

	spec, err := process.NewSpec(absCwd, request.Command, size)
	if err != nil {
		_ = historyWriter.Close()
		_ = r.store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "build_process_spec_failed", UpdatedAt: time.Now().UTC()})
		return CreateSessionResponse{}, apperrors.Runtime("build process spec", err)
	}
	spec.Env = env
	resolved, err := process.ResolveExecutable(spec.Command)
	if err != nil {
		_ = historyWriter.Close()
		_ = r.store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "resolve_executable_failed", UpdatedAt: time.Now().UTC()})
		return CreateSessionResponse{}, apperrors.Runtime("resolve executable", err)
	}
	spec = spec.WithResolvedCommand(resolved)

	if r.logger != nil {
		r.logger.Info("terminal pty start", "session_id", sess.Id, "workspace_id", sess.WorkspaceId, "cols", spec.InitialSize.Cols, "rows", spec.InitialSize.Rows, "command", spec.EffectiveCommand())
	}
	ptySession, err := r.manager.Start(ctx, spec)
	if err != nil {
		_ = historyWriter.Close()
		_ = r.store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "pty_start_failed", UpdatedAt: time.Now().UTC()})
		return CreateSessionResponse{}, apperrors.Runtime("start pty", err)
	}
	if reporter, ok := ptySession.(termpty.ProcessReporter); ok {
		if err := r.store.SaveProcess(sess.WorkspaceId, sess.Id, reporter.ProcessInfo()); err != nil {
			_ = ptySession.Close()
			_ = historyWriter.Close()
			_ = r.store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "save_process_failed", UpdatedAt: time.Now().UTC()})
			return CreateSessionResponse{}, apperrors.Runtime("save process", err)
		}
	}
	if err := r.store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: time.Now().UTC()}); err != nil {
		_ = ptySession.Close()
		_ = historyWriter.Close()
		return CreateSessionResponse{}, apperrors.Runtime("save running state", err)
	}

	if r.logger != nil {
		r.logger.Info("terminal pty started", "session_id", sess.Id, "workspace_id", sess.WorkspaceId)
	}
	runtime := newSessionRuntime(r, sess, ptySession, historyWriter, size)
	r.mu.Lock()
	r.runtimes[sess.Id] = runtime
	r.mu.Unlock()
	runtime.start()

	return CreateSessionResponse{SessionId: sess.Id, WorkspaceId: sess.WorkspaceId, State: string(session.StateRunning)}, nil
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

func (r *Registry) CloseSession(workspaceId string, sessionId string, reason string) (SessionSummary, error) {
	r.mu.Lock()
	runtime := r.runtimes[sessionId]
	r.mu.Unlock()
	if runtime == nil || runtime.session.WorkspaceId != workspaceId {
		return SessionSummary{}, apperrors.Runtime("session not closable", fmt.Errorf("live PTY handle not found for %s/%s", workspaceId, sessionId))
	}
	if err := runtime.closeSession(reason); err != nil {
		return SessionSummary{}, err
	}
	return r.GetSession(workspaceId, sessionId)
}

func (r *Registry) ListWorkspaces() ([]WorkspaceSummary, error) {
	workspaces, _, err := r.store.ListWorkspaces()
	if err != nil {
		return nil, apperrors.Runtime("list workspaces", err)
	}
	out := make([]WorkspaceSummary, 0, len(workspaces))
	for _, ws := range workspaces {
		out = append(out, summaryFromWorkspace(ws))
	}
	return out, nil
}

func (r *Registry) WorkspaceTree() ([]WorkspaceTreeNode, error) {
	workspaces, _, err := r.store.ListWorkspaces()
	if err != nil {
		return nil, apperrors.Runtime("list workspaces", err)
	}
	nodes := make([]WorkspaceTreeNode, 0, len(workspaces))
	for _, ws := range workspaces {
		views, _, err := r.store.ListSessionsByWorkspaceId(ws.Id)
		if err != nil {
			return nil, apperrors.Runtime("list workspace sessions", err)
		}
		nodes = append(nodes, WorkspaceTreeNode{
			Id:        ws.Id,
			Name:      ws.Name,
			Path:      ws.Path,
			SortOrder: ws.SortOrder,
			UpdatedAt: ws.UpdatedAt,
			Children:  r.workspaceSessionSummariesFromViews(views),
		})
	}
	return nodes, nil
}

func (r *Registry) ListSessions() ([]SessionSummary, error) {
	views, _, err := r.store.ListSessions()
	if err != nil {
		return nil, apperrors.Runtime("list sessions", err)
	}
	return r.summariesFromViews(views), nil
}

func (r *Registry) ListSessionsByWorkspaceId(workspaceId string) ([]WorkspaceSessionSummary, error) {
	views, _, err := r.store.ListSessionsByWorkspaceId(workspaceId)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperrors.NotFound("workspace not found", err)
		}
		return nil, apperrors.Runtime("list workspace sessions", err)
	}
	return r.workspaceSessionSummariesFromViews(views), nil
}

func (r *Registry) UpdateSession(workspaceId string, sessionId string, request UpdateSessionRequest) (SessionSummary, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return SessionSummary{}, apperrors.Usage("missing session name")
	}
	view, err := r.sessionView(workspaceId, sessionId)
	if err != nil {
		return SessionSummary{}, err
	}
	updated, err := r.store.UpdateSession(workspaceId, sessionId, func(value *session.Session) error {
		value.Name = name
		value.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return SessionSummary{}, apperrors.Runtime("update session", err)
	}
	view.Session = updated
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

func (r *Registry) UpdateWorkspaceOrder(workspaceIds []string) ([]WorkspaceSummary, error) {
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
	out := make([]WorkspaceSummary, 0, len(updated))
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

func (r *Registry) GetSession(workspaceId string, sessionId string) (SessionSummary, error) {
	view, err := r.sessionView(workspaceId, sessionId)
	if err != nil {
		return SessionSummary{}, err
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

func filterEnv(env []string, denylist []string) []string {
	if len(denylist) == 0 {
		return append([]string(nil), env...)
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		name := entry
		if index := strings.IndexByte(entry, '='); index >= 0 {
			name = entry[:index]
		}
		if envNameDenied(name, denylist) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func envNameDenied(name string, denylist []string) bool {
	for _, pattern := range denylist {
		if pattern == "" {
			continue
		}
		if ok, _ := filepath.Match(pattern, name); ok || strings.EqualFold(name, pattern) || strings.Contains(strings.ToUpper(name), strings.ToUpper(pattern)) {
			return true
		}
	}
	return false
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

func (r *Registry) summariesFromViews(views []session.View) []SessionSummary {
	recoverer := session.Recoverer{Store: r.store}
	out := make([]SessionSummary, 0, len(views))
	for _, view := range views {
		if !r.hasRuntime(view.Session.Id) {
			refreshed, err := recoverer.Refresh(view)
			if err == nil {
				view = refreshed
			}
		}
		out = append(out, r.summaryFromView(view))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func (r *Registry) workspaceSessionSummariesFromViews(views []session.View) []WorkspaceSessionSummary {
	summaries := r.summariesFromViews(views)
	out := make([]WorkspaceSessionSummary, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, WorkspaceSessionSummary{
			Id:              summary.Id,
			Name:            summary.Name,
			Command:         summary.Command,
			Cwd:             summary.Cwd,
			LifecycleState:  summary.LifecycleState,
			AttachmentState: summary.AttachmentState,
			ExitCode:        summary.ExitCode,
			UpdatedAt:       summary.UpdatedAt,
		})
	}
	return out
}

func summaryFromWorkspace(ws workspace.Workspace) WorkspaceSummary {
	return WorkspaceSummary{Id: ws.Id, Name: ws.Name, Path: ws.Path, SortOrder: ws.SortOrder, UpdatedAt: ws.UpdatedAt}
}

func (r *Registry) summaryFromView(view session.View) SessionSummary {
	attachment := AttachmentUnattached
	lifecycleState := view.State.State
	r.mu.Lock()
	if runtime := r.runtimes[view.Session.Id]; runtime != nil {
		attachment = runtime.attachmentState()
		lifecycleState = runtime.lifecycleState()
	}
	r.mu.Unlock()
	return SessionSummary{
		Id:              view.Session.Id,
		Name:            view.Session.Name,
		WorkspaceId:     view.Session.WorkspaceId,
		Command:         view.CommandText,
		Cwd:             view.Session.LaunchCwd,
		LifecycleState:  lifecycleState,
		AttachmentState: attachment,
		ExitCode:        view.ExitCode,
		UpdatedAt:       view.Session.UpdatedAt,
	}
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

func (c *Client) SendControl(message terminalproto.ServerMessage) bool {
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
