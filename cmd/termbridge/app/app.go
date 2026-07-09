package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	agentserver "gitee.com/leoninew/TermBridge-go/internal/agent/application/bootstrap"
	"gitee.com/leoninew/TermBridge-go/internal/agent/application/task/runner"
	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	agentdb "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/database"
	"gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/pty/gopty"
	"gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/storage/history"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
	cloudserver "gitee.com/leoninew/TermBridge-go/internal/cloud/application/bootstrap"
	clouddb "gitee.com/leoninew/TermBridge-go/internal/cloud/infrastructure/database"
	httpserver "gitee.com/leoninew/TermBridge-go/internal/shared/api/server"
	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	"gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config"
	basedb "gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/database"
	logging "gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/logger"
)

type CommandKind string

const (
	CommandExec      CommandKind = "exec"
	CommandWorkspace CommandKind = "workspace"
	CommandSession   CommandKind = "session"
	CommandAgent     CommandKind = "agent"
	CommandCloud     CommandKind = "cloud"
	CommandMigrate   CommandKind = "migrate"
)

type Command struct {
	Kind        CommandKind
	Exec        ExecCommand
	MigrateRole string
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

var runRuntime = func(ctx context.Context, logger *slog.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
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

var runAgentClient = func(ctx context.Context, client *agentapp.Client) error {
	return client.Run(ctx)
}

func Run(ctx context.Context, options Options) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, apperrors.Internal("context cancelled", ctx.Err())
	default:
	}

	command := options.Command.Exec.Command
	loadedConfigFiles := []string{}
	cfg, err := config.Load(config.Options{
		Cwd:     options.Cwd,
		Command: command,
		LoadedConfigFile: func(path string) {
			loadedConfigFiles = append(loadedConfigFiles, path)
		},
	})
	if err != nil {
		return Result{}, err
	}
	var logOutput io.Writer
	if options.Command.Kind == CommandAgent || options.Command.Kind == CommandCloud {
		logOutput = options.Stdout
	}
	logger, err := logging.New(logging.Config{Level: cfg.LogLevel, Format: cfg.LogFormat, Dir: cfg.LogDir, Output: logOutput})
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = logger.Close() }()
	logLoadedConfigFiles(logger.Slog, loadedConfigFiles)

	switch options.Command.Kind {
	case CommandExec:
		logger.Info("termbridge exec command parsed", "cwd", cfg.Cwd, "command", strings.Join(cfg.Command, " "), "config", cfg.DefaultConfigFile)
		return runExec(ctx, cfg, logger.Slog, options)
	case CommandWorkspace:
		return runWorkspaceList(ctx, cfg, options.Stdout)
	case CommandSession:
		return runSessionList(ctx, cfg, options.Stdout)
	case CommandAgent:
		logger.Info("termbridge agent command parsed", "cwd", cfg.Cwd, "server_listen_url", cfg.Local.ListenUrl, "config", cfg.DefaultConfigFile)
		return runAgent(ctx, cfg, logger.Slog, options)
	case CommandCloud:
		logger.Info("termbridge cloud command parsed", "cwd", cfg.Cwd, "server_listen_url", cfg.Cloud.ListenUrl, "config", cfg.DefaultConfigFile)
		return runCloud(ctx, cfg, logger.Slog, options)
	case CommandMigrate:
		logger.Info("termbridge migrate command parsed", "cwd", cfg.Cwd, "config", cfg.DefaultConfigFile, "role", options.Command.MigrateRole)
		return runMigrate(ctx, cfg, options.Command.MigrateRole)
	default:
		return Result{Cwd: cfg.Cwd}, apperrors.Usage("missing command")
	}
}

func logLoadedConfigFiles(logger *slog.Logger, paths []string) {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		logger.Info("加载配置文件", "path", path)
	}
}

func runMigrate(ctx context.Context, cfg config.Config, role string) (Result, error) {
	database := selectRoleDatabase(cfg, role)
	db, err := basedb.Open(ctx, database.Driver, database.SQLite.Path, database.MySQL.Dsn)
	if err != nil {
		return Result{Cwd: cfg.Cwd}, err
	}
	defer func() { _ = db.Close() }()
	switch strings.TrimSpace(role) {
	case "agent":
		if err := agentdb.Migrate(ctx, db.DB, db.Driver); err != nil {
			return Result{Cwd: cfg.Cwd}, err
		}
	case "cloud":
		if err := clouddb.Migrate(ctx, db.DB, db.Driver); err != nil {
			return Result{Cwd: cfg.Cwd}, err
		}
	default:
		return Result{Cwd: cfg.Cwd}, apperrors.Usage("migrate requires role: termbridge migrate agent|cloud")
	}
	return Result{Cwd: cfg.Cwd}, nil
}

func selectRoleDatabase(cfg config.Config, role string) config.DatabaseConfig {
	switch strings.TrimSpace(role) {
	case "cloud":
		return cfg.Cloud.Database
	default:
		return cfg.Local.Database
	}
}

func runExec(ctx context.Context, cfg config.Config, logger *slog.Logger, options Options) (Result, error) {
	agentCfg := agentConfig(cfg)
	result, err := agentserver.RunExec(ctx, agentCfg, logger, agentserver.Options{Stdin: options.Stdin, Stdout: options.Stdout, Stderr: options.Stderr, RunRuntime: runRuntime})
	return Result{Cwd: cfg.Cwd, Command: result.Command, ExitCode: result.ExitCode}, err
}

func runAgent(ctx context.Context, cfg config.Config, logger *slog.Logger, options Options) (Result, error) {
	err := agentserver.Run(ctx, agentConfig(cfg), logger, agentserver.Options{Stdout: options.Stdout, RunBackendServer: runBackendServer, RunClient: runAgentClient})
	return Result{Cwd: cfg.Cwd}, err
}

func runCloud(ctx context.Context, cfg config.Config, logger *slog.Logger, options Options) (Result, error) {
	err := cloudserver.Run(ctx, cloudConfig(cfg), logger, cloudserver.Options{Stdout: options.Stdout, RunBackendServer: runBackendServer})
	return Result{Cwd: cfg.Cwd}, err
}

func runWorkspaceList(ctx context.Context, cfg config.Config, stdout io.Writer) (Result, error) {
	if err := agentserver.RunWorkspaceList(ctx, agentConfig(cfg), stdout); err != nil {
		return Result{Cwd: cfg.Cwd}, err
	}
	return Result{Cwd: cfg.Cwd}, nil
}

func runSessionList(ctx context.Context, cfg config.Config, stdout io.Writer) (Result, error) {
	if err := agentserver.RunSessionList(ctx, agentConfig(cfg), stdout); err != nil {
		return Result{Cwd: cfg.Cwd}, err
	}
	return Result{Cwd: cfg.Cwd}, nil
}

func agentConfig(cfg config.Config) agentserver.Config {
	return agentserver.Config{
		Cwd:     cfg.Cwd,
		Command: append([]string(nil), cfg.Command...),
		LogDir:  cfg.LogDir,
		LogHTTP: cfg.LogHTTP,
		History: history.Config{
			MaxLines:     cfg.History.MaxLines,
			MaxBytes:     cfg.History.MaxBytes,
			MaxLineBytes: cfg.History.MaxLineBytes,
		},
		Terminal: agentserver.TerminalConfig{
			Replay: agentserver.TerminalReplayConfig{
				MaxBytes:   cfg.Terminal.Replay.MaxBytes,
				ChunkBytes: cfg.Terminal.Replay.ChunkBytes,
			},
			Client: agentserver.TerminalClientConfig{Queue: agentserver.TerminalClientQueueConfig{
				MaxMessages: cfg.Terminal.Client.Queue.MaxMessages,
				MaxBytes:    cfg.Terminal.Client.Queue.MaxBytes,
			}},
		},
		Runtime: agentserver.RuntimeConfig{StateDir: cfg.Runtime.StateDir},
		Server: agentserver.ServerConfig{
			ListenURL:          cfg.Local.ListenUrl,
			StaticDir:          cfg.Local.StaticDir,
			ApiBaseUrl:         cfg.Local.ApiBaseUrl,
			CorsAllowedOrigins: cfg.Local.CorsAllowedOrigins,
		},
		Gate: agentserver.GateConfig{API: agentserver.GateApiConfig{ExposeErrors: cfg.Local.ExposeErrors}},
		Database: agentserver.DatabaseConfig{
			Driver: cfg.Local.Database.Driver,
			SQLite: agentserver.SQLiteConfig{Path: cfg.Local.Database.SQLite.Path},
			MySQL:  agentserver.MySQLConfig{Dsn: cfg.Local.Database.MySQL.Dsn},
		},
		Auth: agentserver.AuthConfig{JwtTTL: cfg.Auth.JwtTTL},
		Jwt:  agentserver.JwtConfig{SecretKey: cfg.Jwt.SecretKey},
		Cloud: agentserver.CloudConnectorConfig{
			PublicURL: cfg.Cloud.PublicUrl,
			OAuthClient: agentserver.OAuthClientConfig{
				ClientId:     cfg.Local.OAuth.ClientId,
				ClientSecret: cfg.Local.OAuth.ClientSecret,
				RedirectUrl:  cfg.Local.OAuth.RedirectUrl,
				Scopes:       append([]string(nil), cfg.Local.OAuth.Scopes...),
			},
		},
	}
}

func cloudConfig(cfg config.Config) cloudserver.Config {
	return cloudserver.Config{
		LogHTTP: cfg.LogHTTP,
		Server: cloudserver.ServerConfig{
			ListenURL:          cfg.Cloud.ListenUrl,
			StaticDir:          cfg.Cloud.StaticDir,
			ApiBaseUrl:         cfg.Cloud.ApiBaseUrl,
			CorsAllowedOrigins: cfg.Cloud.CorsAllowedOrigins,
		},
		Gate: cloudserver.GateConfig{API: cloudserver.GateApiConfig{ExposeErrors: cfg.Cloud.ExposeErrors}},
		Database: cloudserver.DatabaseConfig{
			Driver: cfg.Cloud.Database.Driver,
			SQLite: cloudserver.SQLiteConfig{Path: cfg.Cloud.Database.SQLite.Path},
			MySQL:  cloudserver.MySQLConfig{Dsn: cfg.Cloud.Database.MySQL.Dsn},
		},
		Auth: cloudserver.AuthConfig{
			JwtTTL: cfg.Auth.JwtTTL,
			PasswordPolicy: cloudserver.PasswordPolicy{
				MinLength: cfg.Auth.PasswordPolicy.MinLength,
				MaxLength: cfg.Auth.PasswordPolicy.MaxLength,
			},
			Code: cloudserver.CodePolicy{
				Length:         cfg.Auth.Code.Length,
				Ttl:            cfg.Auth.Code.Ttl,
				ResendCooldown: cfg.Auth.Code.ResendCooldown,
				MaxAttempts:    cfg.Auth.Code.MaxAttempts,
			},
			Google: cloudserver.GoogleConfig{
				ClientID:     cfg.Auth.Google.ClientID,
				ClientSecret: cfg.Auth.Google.ClientSecret,
				RedirectUrl:  cfg.Auth.Google.RedirectUrl,
			},
		},
		Jwt: cloudserver.JwtConfig{SecretKey: cfg.Jwt.SecretKey},
		Resend: cloudserver.ResendConfig{
			ApiKey:    cfg.Resend.ApiKey,
			FromEmail: cfg.Resend.FromEmail,
		},
		Cloud: cloudserver.CloudConfig{
			PublicURL: cfg.Cloud.PublicUrl,
			OAuth:     cloudserver.CloudOAuthConfig{Clients: cloudOAuthClients(cfg.Cloud.OAuth.Clients)},
		},
	}
}

func cloudOAuthClients(clients []config.CloudOAuthClientConfig) []cloudserver.CloudOAuthClientConfig {
	out := make([]cloudserver.CloudOAuthClientConfig, 0, len(clients))
	for _, client := range clients {
		out = append(out, cloudserver.CloudOAuthClientConfig{
			ClientId:     client.ClientId,
			ClientSecret: client.ClientSecret,
			RedirectUrl:  client.RedirectUrl,
			Scopes:       append([]string(nil), client.Scopes...),
		})
	}
	return out
}
