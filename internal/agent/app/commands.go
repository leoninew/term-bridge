package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	agentapp "termbridge-go/internal/agent/application"
	agentdevice "termbridge-go/internal/agent/device"
	"termbridge-go/internal/agent/domain/process"
	"termbridge-go/internal/agent/domain/session"
	"termbridge-go/internal/agent/domain/workspace"
	"termbridge-go/internal/agent/history"
	agentdb "termbridge-go/internal/agent/infrastructure/database"
	"termbridge-go/internal/agent/pty/gopty"
	"termbridge-go/internal/agent/runner"
	"termbridge-go/internal/agent/state"
	basedb "termbridge-go/internal/shared/database"
	apperrors "termbridge-go/internal/shared/errors"
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
	db, err := basedb.Open(ctx, cfg.Database.Driver, cfg.Database.SQLite.Path, cfg.Database.MySQL.DSN)
	if err != nil {
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, err
	}
	defer db.Close()
	if err := agentdb.Migrate(ctx, db.DB, db.Driver); err != nil {
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, err
	}
	deviceRepository := agentdevice.NewRepository(db.DB, db.Driver)
	if _, err := deviceRepository.UpsertLocalDevice(ctx, agentdevice.Device{ID: device.Id, Name: device.Name, PublicKey: device.PublicKey}); err != nil {
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("upsert local device", err)
	}
	store := state.NewDBStore(db.DB, db.Driver, filepath.Join(cfg.Runtime.StateDir, "devices", device.Id), device.Id)
	resolver := workspace.Resolver{Store: store}
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
	manager := session.Manager{Store: store}
	sess, err := manager.Create(session.CreateOptions{
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
			if err := store.SaveProcess(sess.WorkspaceId, sess.Id, record); err != nil {
				hookErr = err
				logger.Error("save process record", "error", err)
				return
			}
			if err := store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: time.Now().UTC()}); err != nil {
				hookErr = err
				logger.Error("save running state", "error", err)
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
	if err := store.SaveExit(sess.WorkspaceId, sess.Id, process.ExitRecord{SchemaVersion: 1, ExitCode: runtimeResult.ExitCode, Reason: "user_process_exited", Forced: runtimeResult.Exit.Forced, Closed: runtimeResult.Exit.Closed, StartedAt: startedAt, EndedAt: endedAt, WaitError: waitErr}); err != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "save_exit_failed", UpdatedAt: endedAt})
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("save exit record", err)
	}
	finalState := session.StateStopped
	if runtimeResult.Exit.WaitErr != nil && !runtimeResult.Exit.Stopped && !runtimeResult.Exit.Closed && runtimeResult.Exit.Code == 0 {
		finalState = session.StateFailed
	}
	if err := store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: finalState, Reason: "user_process_exited", UpdatedAt: endedAt}); err != nil {
		return CommandResult{Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("save final state", err)
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
		fmt.Fprintf(stdout, "warning: skipped %s: %v\n", warning.Path, warning.Err)
	}
	sessions, _, _ := store.ListSessions()
	counts := map[string]int{}
	for _, view := range sessions {
		counts[view.Session.WorkspaceId]++
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].UpdatedAt.After(workspaces[j].UpdatedAt) })
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WORKSPACE ID\tNAME\tPATH\tSESSIONS\tUPDATED")
	for _, ws := range workspaces {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", ws.Id, ws.Name, ws.Path, counts[ws.Id], ws.UpdatedAt.Format(time.RFC3339))
	}
	_ = w.Flush()
	return nil
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
		fmt.Fprintf(stdout, "warning: skipped %s: %v\n", warning.Path, warning.Err)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Session.UpdatedAt.After(views[j].Session.UpdatedAt) })
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION ID\tWORKSPACE ID\tSTATE\tEXIT\tCOMMAND\tCWD\tUPDATED")
	for _, view := range views {
		exit := ""
		if view.ExitCode != nil {
			exit = fmt.Sprintf("%d", *view.ExitCode)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", view.Session.Id, view.Session.WorkspaceId, view.State.State, exit, view.CommandText, view.Session.LaunchCwd, view.Session.UpdatedAt.Format(time.RFC3339))
	}
	_ = w.Flush()
	return nil
}

func runtimeStateStore(ctx context.Context, cfg Config) (state.DBStore, func(), error) {
	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: cfg.Runtime.StateDir})
	if err != nil {
		return state.DBStore{}, func() {}, err
	}
	db, err := basedb.Open(ctx, cfg.Database.Driver, cfg.Database.SQLite.Path, cfg.Database.MySQL.DSN)
	if err != nil {
		return state.DBStore{}, func() {}, err
	}
	cleanup := func() { _ = db.Close() }
	if err := agentdb.Migrate(ctx, db.DB, db.Driver); err != nil {
		cleanup()
		return state.DBStore{}, func() {}, err
	}
	deviceRepository := agentdevice.NewRepository(db.DB, db.Driver)
	if _, err := deviceRepository.UpsertLocalDevice(ctx, agentdevice.Device{ID: device.Id, Name: device.Name, PublicKey: device.PublicKey}); err != nil {
		cleanup()
		return state.DBStore{}, func() {}, apperrors.Runtime("upsert local device", err)
	}
	return state.NewDBStore(db.DB, db.Driver, filepath.Join(cfg.Runtime.StateDir, "devices", device.Id), device.Id), cleanup, nil
}

func processEnv() []string {
	return os.Environ()
}
