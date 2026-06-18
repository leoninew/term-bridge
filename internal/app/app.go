package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"termbridge-go/internal/config"
	apperrors "termbridge-go/internal/errors"
	"termbridge-go/internal/history"
	"termbridge-go/internal/identity"
	"termbridge-go/internal/logging"
	"termbridge-go/internal/process"
	"termbridge-go/internal/pty/gopty"
	"termbridge-go/internal/runner"
	"termbridge-go/internal/session"
	"termbridge-go/internal/state"
	"termbridge-go/internal/workspace"
)

type CommandKind string

const (
	CommandExec      CommandKind = "exec"
	CommandWorkspace CommandKind = "workspace"
	CommandSession   CommandKind = "session"
)

type Command struct {
	Kind CommandKind
	Exec ExecCommand
}

type ExecCommand struct {
	Command []string
}

type Options struct {
	Cwd     string
	Command Command
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

type Result struct {
	Cwd      string
	Command  []string
	ExitCode int
}

var runRuntime = func(ctx context.Context, logger *logging.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
	r := runner.CommandRunner{
		Manager:        gopty.NewManager(),
		Logger:         logger,
		InterruptGrace: 1500 * time.Millisecond,
		Hooks:          hooks,
	}
	return r.Run(ctx, spec, streams)
}

func Run(ctx context.Context, options Options) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, apperrors.Internal("context cancelled", ctx.Err())
	default:
	}

	command := options.Command.Exec.Command
	cfg, err := config.Load(config.Options{
		Cwd:     options.Cwd,
		Command: command,
	})
	if err != nil {
		return Result{}, err
	}

	logger, err := logging.New(logging.Config{Level: cfg.LogLevel, Format: cfg.LogFormat, Dir: cfg.LogDir})
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = logger.Close() }()

	switch options.Command.Kind {
	case CommandExec:
		logger.Info("termbridge exec command parsed", "cwd", cfg.Cwd, "command", strings.Join(cfg.Command, " "), "config", cfg.ConfigFile)
		return runExec(ctx, cfg, logger, options)
	case CommandWorkspace:
		return runWorkspaceList(cfg, options.Stdout)
	case CommandSession:
		return runSessionList(cfg, options.Stdout)
	default:
		return Result{Cwd: cfg.Cwd}, apperrors.Usage("missing command")
	}
}

func runExec(ctx context.Context, cfg config.Config, logger *logging.Logger, options Options) (Result, error) {
	store := state.NewStore(cfg.Runtime.StateDir)
	ids := identity.NewULIDGenerator()
	resolver := workspace.Resolver{Store: store, IDs: ids}
	ws, err := resolver.Resolve(cfg.Cwd)
	if err != nil {
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("resolve workspace", err)
	}

	logPath := filepath.Join(cfg.LogDir, "termbridge.log")
	commandRecord := session.CommandRecord{
		Command:     cfg.Command[0],
		Args:        append([]string(nil), cfg.Command[1:]...),
		EnvStrategy: "inherit",
		EnvCount:    len(processEnv()),
	}
	manager := session.Manager{Store: store, IDs: ids}
	sess, err := manager.Create(session.CreateOptions{
		Workspace: ws,
		LaunchCwd: cfg.Cwd,
		Command:   commandRecord,
		History: session.HistoryRecord{
			Path:         "history.log",
			MaxLines:     cfg.History.MaxLines,
			MaxBytes:     cfg.History.MaxBytes,
			MaxLineBytes: cfg.History.MaxLineBytes,
		},
		LogPath: logPath,
	})
	if err != nil {
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("create session", err)
	}

	historyWriter, err := history.NewWriter(store.HistoryPath(sess.WorkspaceKey, sess.ID), history.Config{MaxLines: cfg.History.MaxLines, MaxBytes: cfg.History.MaxBytes, MaxLineBytes: cfg.History.MaxLineBytes})
	if err != nil {
		_ = store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "history_create_failed", UpdatedAt: time.Now().UTC()})
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("create history writer", err)
	}
	defer func() { _ = historyWriter.Close() }()

	spec, err := process.NewSpec(cfg.Cwd, cfg.Command, process.DefaultTerminalSize())
	if err != nil {
		_ = store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "build_process_spec_failed", UpdatedAt: time.Now().UTC()})
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("build process spec", err)
	}

	var hookErr error
	hooks := runner.Hooks{
		OnStarted: func(record process.Record) {
			if err := store.SaveProcess(sess.WorkspaceKey, sess.ID, record); err != nil {
				hookErr = err
				logger.Error("save process record", "error", err)
				return
			}
			if err := store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: time.Now().UTC()}); err != nil {
				hookErr = err
				logger.Error("save running state", "error", err)
			}
		},
		OnStopping: func(_ process.StopMode, reason string) {
			if err := store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopping, Reason: reason, UpdatedAt: time.Now().UTC()}); err != nil {
				hookErr = err
				logger.Error("save stopping state", "error", err)
			}
		},
	}

	stdout := options.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	runtimeResult, err := runRuntime(ctx, logger, spec, runner.IO{Stdin: options.Stdin, Stdout: io.MultiWriter(stdout, historyWriter), Stderr: options.Stderr, TerminalOutput: stdout}, hooks)
	endedAt := time.Now().UTC()
	if err != nil {
		_ = store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "runtime_failed", UpdatedAt: endedAt})
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, err
	}
	if hookErr != nil {
		_ = store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "persistence_failed", UpdatedAt: endedAt})
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("persist session lifecycle", hookErr)
	}
	if err := historyWriter.Close(); err != nil {
		logger.Warn("close history writer", "error", err)
	}
	if historyWriter.Truncated() {
		sess.History.Truncated = true
		sess.UpdatedAt = endedAt
		if err := store.SaveSession(sess); err != nil {
			logger.Warn("save truncated history metadata", "error", err)
		}
	}
	startedAt := runtimeResult.Process.StartedAt
	if startedAt.IsZero() {
		startedAt = sess.CreatedAt
	}
	waitErr := ""
	if runtimeResult.Exit.WaitErr != nil {
		waitErr = runtimeResult.Exit.WaitErr.Error()
	}
	if err := store.SaveExit(sess.WorkspaceKey, sess.ID, process.ExitRecord{SchemaVersion: 1, ExitCode: runtimeResult.ExitCode, Reason: "user_process_exited", Forced: runtimeResult.Exit.Forced, Closed: runtimeResult.Exit.Closed, StartedAt: startedAt, EndedAt: endedAt, WaitError: waitErr}); err != nil {
		_ = store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "save_exit_failed", UpdatedAt: endedAt})
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("save exit record", err)
	}
	if err := store.SaveState(sess.WorkspaceKey, sess.ID, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: "user_process_exited", UpdatedAt: endedAt}); err != nil {
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("save stopped state", err)
	}

	return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...), ExitCode: runtimeResult.ExitCode}, nil
}

func runWorkspaceList(cfg config.Config, stdout io.Writer) (Result, error) {
	if stdout == nil {
		stdout = io.Discard
	}
	store := state.NewStore(cfg.Runtime.StateDir)
	workspaces, warnings, err := store.ListWorkspaces()
	if err != nil {
		return Result{Cwd: cfg.Cwd}, apperrors.Runtime("list workspaces", err)
	}
	for _, warning := range warnings {
		fmt.Fprintf(stdout, "warning: skipped %s: %v\n", warning.Path, warning.Err)
	}
	sessions, _, _ := store.ListSessions()
	counts := map[string]int{}
	for _, view := range sessions {
		counts[view.Session.WorkspaceKey]++
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].UpdatedAt.After(workspaces[j].UpdatedAt) })
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WORKSPACE ID\tKEY\tNAME\tPATH\tSESSIONS\tUPDATED")
	for _, ws := range workspaces {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n", ws.ID, ws.Key, ws.Name, ws.Path, counts[ws.Key], ws.UpdatedAt.Format(time.RFC3339))
	}
	_ = w.Flush()
	return Result{Cwd: cfg.Cwd}, nil
}

func runSessionList(cfg config.Config, stdout io.Writer) (Result, error) {
	if stdout == nil {
		stdout = io.Discard
	}
	store := state.NewStore(cfg.Runtime.StateDir)
	views, warnings, err := store.ListSessions()
	if err != nil {
		return Result{Cwd: cfg.Cwd}, apperrors.Runtime("list sessions", err)
	}
	recoverer := session.Recoverer{Store: store}
	for i, view := range views {
		refreshed, err := recoverer.Refresh(view)
		if err != nil {
			warnings = append(warnings, state.Warning{Path: view.Session.ID, Err: err})
			continue
		}
		views[i] = refreshed
	}
	for _, warning := range warnings {
		fmt.Fprintf(stdout, "warning: skipped %s: %v\n", warning.Path, warning.Err)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Session.UpdatedAt.After(views[j].Session.UpdatedAt) })
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION ID\tWORKSPACE ID\tSTATE\tEXIT\tCOMMAND\tCWD\tUPDATED\tLOG")
	for _, view := range views {
		exit := ""
		if view.ExitCode != nil {
			exit = fmt.Sprintf("%d", *view.ExitCode)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", view.Session.ID, view.Session.WorkspaceID, view.State.State, exit, view.CommandText, view.Session.LaunchCwd, view.Session.UpdatedAt.Format(time.RFC3339), view.Session.LogPath)
	}
	_ = w.Flush()
	return Result{Cwd: cfg.Cwd}, nil
}

func processEnv() []string {
	return os.Environ()
}
