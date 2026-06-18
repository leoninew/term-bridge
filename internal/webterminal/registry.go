package webterminal

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

	"termbridge-go/internal/config"
	apperrors "termbridge-go/internal/errors"
	"termbridge-go/internal/history"
	"termbridge-go/internal/identity"
	"termbridge-go/internal/logging"
	"termbridge-go/internal/process"
	termpty "termbridge-go/internal/pty"
	"termbridge-go/internal/session"
	"termbridge-go/internal/state"
	"termbridge-go/internal/terminalproto"
	"termbridge-go/internal/workspace"
)

const (
	AttachmentUnattached  AttachmentState = "unattached"
	AttachmentAttached    AttachmentState = "attached"
	AttachmentDetached    AttachmentState = "detached"
	AttachmentReattaching AttachmentState = "reattaching"

	DefaultClientQueueSize = 64
)

type AttachmentState string

type Config struct {
	Cwd             string
	Store           state.Store
	LogDir          string
	History         config.HistoryConfig
	Manager         termpty.Manager
	Logger          *logging.Logger
	ClientQueueSize int
}

type Registry struct {
	cwd             string
	store           state.Store
	logDir          string
	historyConfig   config.HistoryConfig
	manager         termpty.Manager
	logger          *logging.Logger
	ids             identity.Generator
	clientQueueSize int

	mu       sync.Mutex
	runtimes map[string]*SessionRuntime
}

type CreateSessionRequest struct {
	Cwd     string   `json:"cwd"`
	Command []string `json:"command"`
	Cols    int      `json:"cols"`
	Rows    int      `json:"rows"`
}

type CreateSessionResponse struct {
	SessionID    string `json:"session_id"`
	WorkspaceID  string `json:"workspace_id"`
	WorkspaceKey string `json:"workspace_key"`
	State        string `json:"state"`
	WSURL        string `json:"ws_url"`
}

type WorkspaceSummary struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionSummary struct {
	ID              string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	WorkspaceKey    string          `json:"workspace_key"`
	Command         string          `json:"command"`
	Cwd             string          `json:"cwd"`
	LifecycleState  session.State   `json:"lifecycle_state"`
	AttachmentState AttachmentState `json:"attachment_state,omitempty"`
	ExitCode        *int            `json:"exit_code,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
	LogPath         string          `json:"log_path"`
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
	id      string
	runtime *SessionRuntime
	queue   chan Outbound
	once    sync.Once
}

func NewRegistry(config Config) *Registry {
	queueSize := config.ClientQueueSize
	if queueSize <= 0 {
		queueSize = DefaultClientQueueSize
	}
	return &Registry{
		cwd:             config.Cwd,
		store:           config.Store,
		logDir:          config.LogDir,
		historyConfig:   config.History,
		manager:         config.Manager,
		logger:          config.Logger,
		ids:             identity.NewULIDGenerator(),
		clientQueueSize: queueSize,
		runtimes:        map[string]*SessionRuntime{},
	}
}

func (r *Registry) CreateSession(ctx context.Context, request CreateSessionRequest) (CreateSessionResponse, error) {
	if len(request.Command) == 0 {
		return CreateSessionResponse{}, apperrors.Usage("missing session command")
	}
	cwd := request.Cwd
	if strings.TrimSpace(cwd) == "" {
		cwd = r.cwd
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return CreateSessionResponse{}, apperrors.Config("resolve session cwd", err)
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

	resolver := workspace.Resolver{Store: r.store, IDs: r.ids}
	ws, err := resolver.Resolve(absCwd)
	if err != nil {
		return CreateSessionResponse{}, apperrors.Runtime("resolve workspace", err)
	}

	commandRecord := session.CommandRecord{
		Command:     request.Command[0],
		Args:        append([]string(nil), request.Command[1:]...),
		EnvStrategy: "inherit",
		EnvCount:    len(os.Environ()),
	}
	manager := session.Manager{Store: r.store, IDs: r.ids}
	sess, err := manager.Create(session.CreateOptions{
		Workspace: ws,
		LaunchCwd: absCwd,
		Command:   commandRecord,
		History: session.HistoryRecord{
			Path:         "history.log",
			MaxLines:     r.historyConfig.MaxLines,
			MaxBytes:     r.historyConfig.MaxBytes,
			MaxLineBytes: r.historyConfig.MaxLineBytes,
		},
		LogPath: filepath.Join(r.logDir, "termbridge.log"),
	})
	if err != nil {
		return CreateSessionResponse{}, apperrors.Runtime("create session", err)
	}

	historyWriter, err := history.NewWriter(r.store.HistoryPath(sess.WorkspaceKey, sess.ID), history.Config{MaxLines: r.historyConfig.MaxLines, MaxBytes: r.historyConfig.MaxBytes, MaxLineBytes: r.historyConfig.MaxLineBytes})
	if err != nil {
		_ = r.store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "history_create_failed", UpdatedAt: time.Now().UTC()})
		return CreateSessionResponse{}, apperrors.Runtime("create history writer", err)
	}

	spec, err := process.NewSpec(absCwd, request.Command, size)
	if err != nil {
		_ = historyWriter.Close()
		_ = r.store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "build_process_spec_failed", UpdatedAt: time.Now().UTC()})
		return CreateSessionResponse{}, apperrors.Runtime("build process spec", err)
	}
	resolved, err := process.ResolveExecutable(spec.Command)
	if err != nil {
		_ = historyWriter.Close()
		_ = r.store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "resolve_executable_failed", UpdatedAt: time.Now().UTC()})
		return CreateSessionResponse{}, apperrors.Runtime("resolve executable", err)
	}
	spec = spec.WithResolvedCommand(resolved)

	ptySession, err := r.manager.Start(ctx, spec)
	if err != nil {
		_ = historyWriter.Close()
		_ = r.store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "pty_start_failed", UpdatedAt: time.Now().UTC()})
		return CreateSessionResponse{}, apperrors.Runtime("start pty", err)
	}
	if reporter, ok := ptySession.(termpty.ProcessReporter); ok {
		if err := r.store.SaveProcess(sess.WorkspaceKey, sess.ID, reporter.ProcessInfo()); err != nil {
			_ = ptySession.Close()
			_ = historyWriter.Close()
			_ = r.store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "save_process_failed", UpdatedAt: time.Now().UTC()})
			return CreateSessionResponse{}, apperrors.Runtime("save process", err)
		}
	}
	if err := r.store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: time.Now().UTC()}); err != nil {
		_ = ptySession.Close()
		_ = historyWriter.Close()
		return CreateSessionResponse{}, apperrors.Runtime("save running state", err)
	}

	runtime := newSessionRuntime(r, sess, ptySession, historyWriter)
	r.mu.Lock()
	r.runtimes[sess.ID] = runtime
	r.mu.Unlock()
	runtime.start()

	return CreateSessionResponse{SessionID: sess.ID, WorkspaceID: sess.WorkspaceID, WorkspaceKey: sess.WorkspaceKey, State: string(session.StateRunning), WSURL: "/api/sessions/" + sess.ID + "/ws"}, nil
}

func (r *Registry) Attach(sessionID string) (*Client, error) {
	r.mu.Lock()
	runtime := r.runtimes[sessionID]
	r.mu.Unlock()
	if runtime == nil {
		return nil, apperrors.Runtime("session not attachable", fmt.Errorf("live PTY handle not found for %s", sessionID))
	}
	return runtime.attach()
}

func (r *Registry) ListWorkspaces() ([]WorkspaceSummary, error) {
	workspaces, _, err := r.store.ListWorkspaces()
	if err != nil {
		return nil, apperrors.Runtime("list workspaces", err)
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].UpdatedAt.After(workspaces[j].UpdatedAt) })
	out := make([]WorkspaceSummary, 0, len(workspaces))
	for _, ws := range workspaces {
		out = append(out, WorkspaceSummary{ID: ws.ID, Key: ws.Key, Name: ws.Name, Path: ws.Path, UpdatedAt: ws.UpdatedAt})
	}
	return out, nil
}

func (r *Registry) ListSessions() ([]SessionSummary, error) {
	views, _, err := r.store.ListSessions()
	if err != nil {
		return nil, apperrors.Runtime("list sessions", err)
	}
	recoverer := session.Recoverer{Store: r.store}
	out := make([]SessionSummary, 0, len(views))
	for _, view := range views {
		refreshed, err := recoverer.Refresh(view)
		if err == nil {
			view = refreshed
		}
		out = append(out, r.summaryFromView(view))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (r *Registry) GetSession(sessionID string) (SessionSummary, error) {
	views, _, err := r.store.ListSessions()
	if err != nil {
		return SessionSummary{}, apperrors.Runtime("list sessions", err)
	}
	for _, view := range views {
		if view.Session.ID == sessionID {
			return r.summaryFromView(view), nil
		}
	}
	return SessionSummary{}, apperrors.Runtime("load session", os.ErrNotExist)
}

func (r *Registry) History(sessionID string) ([]byte, error) {
	views, _, err := r.store.ListSessions()
	if err != nil {
		return nil, apperrors.Runtime("list sessions", err)
	}
	for _, view := range views {
		if view.Session.ID == sessionID {
			data, err := os.ReadFile(r.store.HistoryPath(view.Session.WorkspaceKey, view.Session.ID))
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			if err != nil {
				return nil, apperrors.Runtime("read history", err)
			}
			return data, nil
		}
	}
	return nil, apperrors.Runtime("load session", os.ErrNotExist)
}

func (r *Registry) removeRuntime(sessionID string) {
	r.mu.Lock()
	delete(r.runtimes, sessionID)
	r.mu.Unlock()
}

func (r *Registry) summaryFromView(view session.View) SessionSummary {
	attachment := AttachmentUnattached
	r.mu.Lock()
	if runtime := r.runtimes[view.Session.ID]; runtime != nil {
		attachment = runtime.attachmentState()
	}
	r.mu.Unlock()
	return SessionSummary{
		ID:              view.Session.ID,
		WorkspaceID:     view.Session.WorkspaceID,
		WorkspaceKey:    view.Session.WorkspaceKey,
		Command:         view.CommandText,
		Cwd:             view.Session.LaunchCwd,
		LifecycleState:  view.State.State,
		AttachmentState: attachment,
		ExitCode:        view.ExitCode,
		UpdatedAt:       view.Session.UpdatedAt,
		LogPath:         view.Session.LogPath,
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

func (c *Client) enqueue(outbound Outbound) bool {
	select {
	case c.queue <- outbound:
		return true
	default:
		return false
	}
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
