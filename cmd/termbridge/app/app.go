package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	agentserver "termbridge-go/internal/agent/application/bootstrap"
	"termbridge-go/internal/agent/application/task/runner"
	agentapp "termbridge-go/internal/agent/application/user"
	agentdb "termbridge-go/internal/agent/infrastructure/database"
	"termbridge-go/internal/agent/infrastructure/pty/gopty"
	"termbridge-go/internal/agent/infrastructure/storage/history"
	"termbridge-go/internal/agent/model/task/process"
	cloudserver "termbridge-go/internal/cloud/application/bootstrap"
	clouddb "termbridge-go/internal/cloud/infrastructure/database"
	httpserver "termbridge-go/internal/shared/api/server"
	apperrors "termbridge-go/internal/shared/common/errors"
	"termbridge-go/internal/shared/infrastructure/config"
	basedb "termbridge-go/internal/shared/infrastructure/database"
	"termbridge-go/internal/shared/infrastructure/logger"
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
		logger.Info("termbridge agent command parsed", "cwd", cfg.Cwd, "server_listen_url", cfg.Agent.ListenUrl, "config", cfg.DefaultConfigFile)
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
		return cfg.Agent.Database
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
		LogHTTP: agentserver.LogHTTPConfig{
			RequestBodyLimit:  cfg.LogHTTP.RequestBodyLimit,
			ResponseBodyLimit: cfg.LogHTTP.ResponseBodyLimit,
		},
		History: history.Config{
			MaxLines:     cfg.History.MaxLines,
			MaxBytes:     cfg.History.MaxBytes,
			MaxLineBytes: cfg.History.MaxLineBytes,
		},
		Runtime: agentserver.RuntimeConfig{StateDir: cfg.Runtime.StateDir},
		Server: agentserver.ServerConfig{
			ListenURL:          cfg.Agent.ListenUrl,
			StaticDir:          cfg.Agent.StaticDir,
			ApiBaseUrl:         cfg.Agent.ApiBaseUrl,
			CorsAllowedOrigins: roleCorsAllowedOrigins(cfg.Agent.PublicUrl, cfg.Agent.CorsAllowedOrigins),
		},
		Gate: agentserver.GateConfig{API: agentserver.GateApiConfig{ExposeErrors: cfg.Agent.ExposeErrors}},
		Database: agentserver.DatabaseConfig{
			Driver: cfg.Agent.Database.Driver,
			SQLite: agentserver.SQLiteConfig{Path: cfg.Agent.Database.SQLite.Path},
			MySQL:  agentserver.MySQLConfig{Dsn: cfg.Agent.Database.MySQL.Dsn},
		},
		Auth: agentserver.AuthConfig{JwtTTL: cfg.Auth.JwtTTL},
		Jwt:  agentserver.JwtConfig{SecretKey: cfg.Jwt.SecretKey},
		Cloud: agentserver.CloudConnectorConfig{
			GateURL: cfg.Cloud.GateUrl,
		},
	}
}

func roleCorsAllowedOrigins(publicURL string, configuredOrigins []string) []string {
	origins := make([]string, 0, len(configuredOrigins)+1)
	if origin := strings.TrimRight(strings.TrimSpace(publicURL), "/"); origin != "" {
		origins = append(origins, origin)
	}
	origins = append(origins, configuredOrigins...)
	return origins
}

func cloudConfig(cfg config.Config) cloudserver.Config {
	return cloudserver.Config{
		LogHTTP: cloudserver.LogHTTPConfig{
			RequestBodyLimit:  cfg.LogHTTP.RequestBodyLimit,
			ResponseBodyLimit: cfg.LogHTTP.ResponseBodyLimit,
		},
		Server: cloudserver.ServerConfig{
			ListenURL:          cfg.Cloud.ListenUrl,
			StaticDir:          cfg.Cloud.StaticDir,
			ApiBaseUrl:         cfg.Cloud.ApiBaseUrl,
			CorsAllowedOrigins: roleCorsAllowedOrigins(cfg.Cloud.PublicUrl, cfg.Cloud.CorsAllowedOrigins),
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
			GateURL: cfg.Cloud.GateUrl,
		},
	}
}
