package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"termbridge-go/internal/application/agent"
	"termbridge-go/internal/application/runner"
	terminalapp "termbridge-go/internal/application/terminal"
	"termbridge-go/internal/domain/identity"
	"termbridge-go/internal/domain/process"
	"termbridge-go/internal/domain/session"
	"termbridge-go/internal/domain/workspace"
	"termbridge-go/internal/infrastructure/config"
	apperrors "termbridge-go/internal/infrastructure/errors"
	"termbridge-go/internal/infrastructure/history"
	"termbridge-go/internal/infrastructure/logging"
	"termbridge-go/internal/infrastructure/pty/gopty"
	"termbridge-go/internal/infrastructure/repository/state"
	"termbridge-go/internal/transport/http/gatewayapi"
	httpserver "termbridge-go/internal/transport/http/server"
)

type CommandKind string

const (
	CommandExec      CommandKind = "exec"
	CommandWorkspace CommandKind = "workspace"
	CommandSession   CommandKind = "session"
	CommandServe     CommandKind = "serve"
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

var runBackendServer = func(ctx context.Context, server *httpserver.Server, onListening func(httpserver.Info)) error {
	listener, info, err := server.Listen()
	if err != nil {
		return err
	}
	if onListening != nil {
		onListening(info)
	}
	return server.Serve(ctx, listener)
}

var runAgentClient = func(ctx context.Context, client *agent.Client) error {
	return client.Run(ctx)
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
	bootstrap := config.BootstrapResult{}
	if options.Command.Kind == CommandServe {
		cfg, bootstrap, err = config.EnsureLocalIdentity(cfg)
		if err != nil {
			return Result{}, err
		}
	}

	var logOutput io.Writer
	if options.Command.Kind == CommandServe {
		logOutput = options.Stdout
	}
	logger, err := logging.New(logging.Config{Level: cfg.LogLevel, Format: cfg.LogFormat, Dir: cfg.LogDir, Output: logOutput})
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
	case CommandServe:
		logger.Info("termbridge serve command parsed", "cwd", cfg.Cwd, "gate_listen_url", cfg.Gate.ListenUrl, "agent_connect_url", cfg.Agent.ConnectUrl, "agent_device_id", cfg.Agent.DeviceId, "agent_device_name", cfg.Agent.DeviceName, "config", cfg.ConfigFile)
		return runServe(ctx, cfg, bootstrap, logger, options)
	default:
		return Result{Cwd: cfg.Cwd}, apperrors.Usage("missing command")
	}
}

func runExec(ctx context.Context, cfg config.Config, logger *logging.Logger, options Options) (Result, error) {
	store := state.NewStore(cfg.Runtime.StateDir)
	ids := identity.NewUlidGenerator()
	resolver := workspace.Resolver{Store: store, Ids: ids}
	ws, err := resolver.Resolve(cfg.Cwd)
	if err != nil {
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("resolve workspace", err)
	}

	commandRecord := session.CommandRecord{
		Command:     cfg.Command[0],
		Args:        append([]string(nil), cfg.Command[1:]...),
		EnvStrategy: "inherit",
		EnvCount:    len(processEnv()),
	}
	manager := session.Manager{Store: store, Ids: ids}
	sess, err := manager.Create(session.CreateOptions{
		Workspace: ws,
		Name:      strings.Join(cfg.Command, " "),
		LaunchCwd: cfg.Cwd,
		Command:   commandRecord,
		History:   session.HistoryRecord{Path: "history.log", MaxLines: cfg.History.MaxLines, MaxBytes: cfg.History.MaxBytes, MaxLineBytes: cfg.History.MaxLineBytes},
	})
	if err != nil {
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("create session", err)
	}

	historyWriter, err := history.NewWriter(store.HistoryPath(sess.WorkspaceId, sess.Id), history.Config{MaxLines: cfg.History.MaxLines, MaxBytes: cfg.History.MaxBytes, MaxLineBytes: cfg.History.MaxLineBytes})
	if err != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "history_create_failed", UpdatedAt: time.Now().UTC()})
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("create history writer", err)
	}
	defer func() { _ = historyWriter.Close() }()

	spec, err := process.NewSpec(cfg.Cwd, cfg.Command, process.DefaultTerminalSize())
	if err != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "build_process_spec_failed", UpdatedAt: time.Now().UTC()})
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("build process spec", err)
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
	runtimeResult, err := runRuntime(ctx, logger, spec, runner.IO{Stdin: options.Stdin, Stdout: io.MultiWriter(stdout, historyWriter), Stderr: options.Stderr, TerminalOutput: stdout}, hooks)
	endedAt := time.Now().UTC()
	if err != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "runtime_failed", UpdatedAt: endedAt})
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, err
	}
	if hookErr != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "persistence_failed", UpdatedAt: endedAt})
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
	if err := store.SaveExit(sess.WorkspaceId, sess.Id, process.ExitRecord{SchemaVersion: 1, ExitCode: runtimeResult.ExitCode, Reason: "user_process_exited", Forced: runtimeResult.Exit.Forced, Closed: runtimeResult.Exit.Closed, StartedAt: startedAt, EndedAt: endedAt, WaitError: waitErr}); err != nil {
		_ = store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateFailed, Reason: "save_exit_failed", UpdatedAt: endedAt})
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("save exit record", err)
	}
	finalState := session.StateStopped
	if runtimeResult.Exit.WaitErr != nil && !runtimeResult.Exit.Stopped && !runtimeResult.Exit.Closed && runtimeResult.Exit.Code == 0 {
		finalState = session.StateFailed
	}
	if err := store.SaveState(sess.WorkspaceId, sess.Id, session.StateRecord{SchemaVersion: session.SchemaVersion, State: finalState, Reason: "user_process_exited", UpdatedAt: endedAt}); err != nil {
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, apperrors.Runtime("save final state", err)
	}

	return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...), ExitCode: runtimeResult.ExitCode}, nil
}

func runServe(ctx context.Context, cfg config.Config, bootstrap config.BootstrapResult, logger *logging.Logger, options Options) (Result, error) {
	stdout := options.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	if bootstrap.Generated {
		fmt.Fprintf(stdout, "TermBridge generated login credentials in %s\n", bootstrap.ConfigFile)
		fmt.Fprintf(stdout, "Username: %s\n", bootstrap.Username)
		fmt.Fprintf(stdout, "Password: %s\n", bootstrap.Password)
	}
	registry := newWebTerminalRegistry(cfg, logger)
	gatewayHandler := gatewayapi.New(gatewayapi.Config{Username: cfg.Auth.Username, Password: cfg.Auth.Password, JWTSecret: cfg.JWT.SecretKey, AllowedOrigins: cfg.Gate.Browser.AllowedOrigins, DebugErrors: cfg.Gate.API.ExposeErrors, Logger: logger.Slog})
	server := httpserver.New(httpserver.Config{ServerUrl: cfg.Gate.ListenUrl, Logger: logger.Slog, RequestBodyLimit: cfg.LogHTTP.RequestBodyLimit, ResponseBodyLimit: cfg.LogHTTP.ResponseBodyLimit}, gatewayHandler)
	client := agent.New(agent.Config{ConnectUrl: cfg.Agent.ConnectUrl, Username: cfg.Auth.Username, Password: cfg.Auth.Password, DeviceId: cfg.Agent.DeviceId, DeviceName: cfg.Agent.DeviceName, StateDir: cfg.Runtime.StateDir, Runtime: agent.WebTerminalAccess{Registry: registry}, Logger: logger.Slog})

	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	backendReady := make(chan struct{})
	errCh := make(chan error, 2)

	go func() {
		err := runBackendServer(serveCtx, server, func(info httpserver.Info) {
			fmt.Fprintf(stdout, "TermBridge serve listening on %s\n", info.Url)
			close(backendReady)
		})
		errCh <- normalizeServeError(err)
	}()

	select {
	case <-backendReady:
	case err := <-errCh:
		cancel()
		if err != nil {
			return Result{Cwd: cfg.Cwd}, err
		}
		return Result{Cwd: cfg.Cwd}, nil
	case <-ctx.Done():
		cancel()
		return Result{Cwd: cfg.Cwd}, nil
	}

	fmt.Fprintf(stdout, "TermBridge agent connector targeting %s\n", cfg.Agent.ConnectUrl)
	go func() {
		errCh <- normalizeServeError(runAgentClient(serveCtx, client))
	}()

	select {
	case err := <-errCh:
		cancel()
		if err != nil {
			return Result{Cwd: cfg.Cwd}, err
		}
		return Result{Cwd: cfg.Cwd}, nil
	case <-ctx.Done():
		cancel()
		return Result{Cwd: cfg.Cwd}, nil
	}
}

func normalizeServeError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func newWebTerminalRegistry(cfg config.Config, logger *logging.Logger) *terminalapp.Registry {
	return terminalapp.NewRegistry(terminalapp.Config{
		Cwd:     cfg.Cwd,
		Store:   state.NewDeviceStore(cfg.Runtime.StateDir, cfg.Agent.DeviceId),
		LogDir:  cfg.LogDir,
		History: cfg.History,
		Manager: gopty.NewManager(),
		Logger:  logger,
	})
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
		counts[view.Session.WorkspaceId]++
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].UpdatedAt.After(workspaces[j].UpdatedAt) })
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WORKSPACE ID\tNAME\tPATH\tSESSIONS\tUPDATED")
	for _, ws := range workspaces {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", ws.Id, ws.Name, ws.Path, counts[ws.Id], ws.UpdatedAt.Format(time.RFC3339))
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
	return Result{Cwd: cfg.Cwd}, nil
}

func processEnv() []string {
	return os.Environ()
}
