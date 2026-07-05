package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"termbridge-go/internal/agent/application/task/runner"
	sessionapp "termbridge-go/internal/agent/application/task/session"
	workspaceapp "termbridge-go/internal/agent/application/task/workspace"
	agentapp "termbridge-go/internal/agent/application/user"
	agentdb "termbridge-go/internal/agent/infrastructure/database"
	"termbridge-go/internal/agent/infrastructure/pty/gopty"
	"termbridge-go/internal/agent/infrastructure/storage/history"
	"termbridge-go/internal/agent/model/task/process"
	"termbridge-go/internal/agent/model/task/session"
	"termbridge-go/internal/agent/repository/task/state"
	apperrors "termbridge-go/internal/shared/common/errors"
	basedb "termbridge-go/internal/shared/infrastructure/database"
)

type CommandResult struct {
	Command  []string
	ExitCode int
}

type RuntimeRunner func(ctx context.Context, logger *slog.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error)

func defaultRuntimeRunner(ctx context.Context, logger *slog.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
	r := runner.CommandRunner{
		Manager:        gopty.NewManager(),
		Logger:         logger,
		InterruptGrace: 1500 * time.Millisecond,
		Hooks:          hooks,
	}
	return r.Run(ctx, spec, streams)
}

func RunExec(ctx context.Context, cfg Config, logger *slog.Logger, options Options) (CommandResult, error) {
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: cfg.Runtime.StateDir})
	if err != nil {
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, err
	}
	db, err := basedb.Open(ctx, cfg.Database.Driver, cfg.Database.SQLite.Path, cfg.Database.MySQL.Dsn)
	if err != nil {
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, err
	}
	defer func() { _ = db.Close() }()
	if err := agentdb.Migrate(ctx, db.DB, db.Driver); err != nil {
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, err
	}
	store := state.NewDbStore(db.DB, db.Driver, cfg.Runtime.StateDir, device.Id)
	resolver := workspaceapp.Resolver{Store: store}
	ws, err := resolver.Resolve(cfg.Cwd)
	if err != nil {
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("resolve workspace", err)
	}

	commandRecord := session.CommandRecord{
		Command:     cfg.Command[0],
		Args:        append([]string(nil), cfg.Command[1:]...),
		EnvStrategy: "inherit",
		EnvCount:    len(processEnv()),
	}
	manager := sessionapp.Manager{Store: store}
	sess, err := manager.Create(sessionapp.CreateOptions{
		Workspace: ws,
		Name:      strings.Join(cfg.Command, " "),
		LaunchCwd: cfg.Cwd,
		Command:   commandRecord,
		History:   session.HistoryRecord{Path: "history.log", MaxLines: cfg.History.MaxLines, MaxBytes: cfg.History.MaxBytes, MaxLineBytes: cfg.History.MaxLineBytes},
	})
	if err != nil {
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("create session", err)
	}

	historyWriter, err := history.NewWriter(store.HistoryPath(sess.WorkspaceId, sess.Id), history.Config{MaxLines: cfg.History.MaxLines, MaxBytes: cfg.History.MaxBytes, MaxLineBytes: cfg.History.MaxLineBytes})
	if err != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "history_create_failed", UpdatedAt: time.Now().UTC()})
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("create history writer", err)
	}
	defer func() { _ = historyWriter.Close() }()

	spec, err := process.NewSpec(cfg.Cwd, cfg.Command, process.DefaultTerminalSize())
	if err != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "build_process_spec_failed", UpdatedAt: time.Now().UTC()})
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("build process spec", err)
	}

	var hookErr error
	hooks := runner.Hooks{
		OnStarted: func(record process.Record) {
			updatedAt := record.StartedAt
			if updatedAt.IsZero() {
				updatedAt = time.Now().UTC()
			}
			if err := store.SaveProcessState(sess.WorkspaceId, sess.Id, record, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: updatedAt}); err != nil {
				hookErr = err
				logger.Error("save process state", "error", err)
			}
		},
	}

	stdout := options.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	runnerFunc := options.RunRuntime
	if runnerFunc == nil {
		runnerFunc = defaultRuntimeRunner
	}
	runtimeResult, err := runnerFunc(ctx, logger, spec, runner.IO{Stdin: options.Stdin, Stdout: io.MultiWriter(stdout, historyWriter), Stderr: options.Stderr, TerminalOutput: stdout}, hooks)
	endedAt := time.Now().UTC()
	if err != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "runtime_failed", UpdatedAt: endedAt})
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, err
	}
	if hookErr != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "persistence_failed", UpdatedAt: endedAt})
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("persist session lifecycle", hookErr)
	}
	if err := historyWriter.Close(); err != nil {
		logger.Warn("close history writer", "error", err)
	}
	sessionUpdate := sess
	truncatedHistory := historyWriter.Truncated()
	if truncatedHistory {
		sessionUpdate.History.Truncated = true
		sessionUpdate.UpdatedAt = endedAt
	}
	startedAt := runtimeResult.Process.StartedAt
	if startedAt.IsZero() {
		startedAt = sess.CreatedAt
	}
	waitErr := ""
	if runtimeResult.Exit.WaitErr != nil {
		waitErr = runtimeResult.Exit.WaitErr.Error()
	}
	finalState := session.StateStopped
	if runtimeResult.Exit.WaitErr != nil && !runtimeResult.Exit.Stopped && !runtimeResult.Exit.Closed && runtimeResult.Exit.Code == 0 {
		finalState = session.StateFailed
	}
	exitRecord := process.ExitRecord{SchemaVersion: 1, ExitCode: runtimeResult.ExitCode, Reason: "user_process_exited", Forced: runtimeResult.Exit.Forced, Closed: runtimeResult.Exit.Closed, StartedAt: startedAt, EndedAt: endedAt, WaitError: waitErr}
	stateRecord := session.StateRecord{SchemaVersion: session.SchemaVersion, State: finalState, Reason: "user_process_exited", UpdatedAt: endedAt}
	if truncatedHistory {
		if err := store.SaveSessionExitState(sessionUpdate, exitRecord, stateRecord); err != nil {
			_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "save_exit_failed", UpdatedAt: endedAt})
			return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("save session exit state", err)
		}
	} else if err := store.SaveExitState(sess.WorkspaceId, sess.Id, exitRecord, stateRecord); err != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "save_exit_failed", UpdatedAt: endedAt})
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("save exit state", err)
	}

	return CommandResult{Command: append([]string(nil), cfg.Command...), ExitCode: runtimeResult.ExitCode}, nil
}

func RunWorkspaceList(ctx context.Context, cfg Config, stdout io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}
	store, cleanup, err := runtimeStateStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	workspaces, warnings, err := store.ListWorkspaces()
	if err != nil {
		return apperrors.Runtime("list workspaces", err)
	}
	for _, warning := range warnings {
		if _, err := fmt.Fprintf(stdout, "warning: skipped %s: %v\n", warning.Path, warning.Err); err != nil {
			return err
		}
	}
	sessions, _, _ := store.ListSessions()
	counts := map[string]int{}
	for _, view := range sessions {
		counts[view.Session.WorkspaceId]++
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].UpdatedAt.After(workspaces[j].UpdatedAt) })
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "WORKSPACE Id\tNAME\tPATH\tSESSIONS\tUPDATED"); err != nil {
		return err
	}
	for _, ws := range workspaces {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", ws.Id, ws.Name, ws.Path, counts[ws.Id], ws.UpdatedAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return w.Flush()
}

func RunSessionList(ctx context.Context, cfg Config, stdout io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}
	store, cleanup, err := runtimeStateStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	views, warnings, err := store.ListSessions()
	if err != nil {
		return apperrors.Runtime("list sessions", err)
	}
	recoverer := session.Recoverer{Store: store}
	for i, view := range views {
		refreshed, err := recoverer.Refresh(view)
		if err != nil {
			warnings = append(warnings, state.Warning{Path: view.Session.Id, Err: err})
			continue
		}
		views[i] = refreshed
	}
	for _, warning := range warnings {
		if _, err := fmt.Fprintf(stdout, "warning: skipped %s: %v\n", warning.Path, warning.Err); err != nil {
			return err
		}
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Session.UpdatedAt.After(views[j].Session.UpdatedAt) })
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "SESSION Id\tWORKSPACE Id\tSTATE\tEXIT\tCOMMAND\tCWD\tUPDATED"); err != nil {
		return err
	}
	for _, view := range views {
		exit := ""
		if view.ExitCode != nil {
			exit = fmt.Sprintf("%d", *view.ExitCode)
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", view.Session.Id, view.Session.WorkspaceId, view.State.State, exit, view.CommandText, view.Session.LaunchCwd, view.Session.UpdatedAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return w.Flush()
}

func runtimeStateStore(ctx context.Context, cfg Config) (state.DbStore, func(), error) {
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: cfg.Runtime.StateDir})
	if err != nil {
		return state.DbStore{}, func() {}, err
	}
	db, err := basedb.Open(ctx, cfg.Database.Driver, cfg.Database.SQLite.Path, cfg.Database.MySQL.Dsn)
	if err != nil {
		return state.DbStore{}, func() {}, err
	}
	cleanup := func() { _ = db.Close() }
	if err := agentdb.Migrate(ctx, db.DB, db.Driver); err != nil {
		cleanup()
		return state.DbStore{}, func() {}, err
	}
	return state.NewDbStore(db.DB, db.Driver, cfg.Runtime.StateDir, device.Id), cleanup, nil
}

func processEnv() []string {
	return os.Environ()
}
