package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	agentapi "gitee.com/leoninew/TermBridge-go/internal/agent/api/handler"
	shortcutapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/shortcut"
	terminalapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/task/terminal"
	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	cloudapi "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/cloudapi"
	agentdb "gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/database"
	"gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/pty/gopty"
	"gitee.com/leoninew/TermBridge-go/internal/agent/repository/task/state"
	cloud "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	httpserver "gitee.com/leoninew/TermBridge-go/internal/shared/api/server"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	basedb "gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/database"
)

type Options struct {
	Stdin            io.Reader
	Stdout           io.Writer
	Stderr           io.Writer
	RunBackendServer func(context.Context, *httpserver.Server, func(httpserver.Info)) error
	RunClient        func(context.Context, *agentapp.Client) error
	RunRuntime       RuntimeRunner
}

func Run(ctx context.Context, cfg Config, logger *slog.Logger, options Options) error {
	stdout := options.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	db, err := basedb.Open(ctx, cfg.Database.Driver, cfg.Database.SQLite.Path, cfg.Database.MySQL.Dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := agentdb.Migrate(ctx, db.DB, db.Driver); err != nil {
		return err
	}

	tokens, err := sharedauth.NewTokenServiceFromBase64Key(cfg.Jwt.SecretKey, cfg.Auth.JwtTTL)
	if err != nil {
		return apperrors.Config("invalid jwt.secret_key", err)
	}
	authService := agentapi.NewLocalAuthService(tokens)

	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: cfg.Runtime.StateDir})
	if err != nil {
		return err
	}
	runtimeStore := state.NewDbStore(db.DB, db.Driver, cfg.Runtime.StateDir, device.Id)
	runtimeAccess := agentapp.WebTerminalAccess{Registry: newWebTerminalRegistry(cfg, logger, runtimeStore), Shortcuts: shortcutapp.NewService(runtimeStore)}

	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	backendReady := make(chan struct{})
	errCh := make(chan error, 3)
	cloudConnector := newCloudConnectorLifecycle(serveCtx, cfg, runtimeAccess, device, logger, options, stdout)
	cloudService := agentapp.NewCloudService(
		cloudapi.New(cloudapi.Config{ApiBaseUrl: cfg.Cloud.ApiBaseUrl, HttpClient: http.DefaultClient}),
		agentapp.CloudServiceConfig{
			PublicUrl:   cfg.Cloud.PublicURL,
			OAuthClient: agentapp.OAuthClientConfig{ClientId: cfg.Cloud.OAuthClient.ClientId, ClientSecret: cfg.Cloud.OAuthClient.ClientSecret, RedirectUrl: cfg.Cloud.OAuthClient.RedirectUrl, Scopes: append([]string(nil), cfg.Cloud.OAuthClient.Scopes...)},
		},
	)

	agentHandler := agentapi.New(agentapi.Config{DebugErrors: cfg.Gate.API.ExposeErrors, Logger: logger, AuthService: authService, CloudService: cloudService, LocalDevice: device, LocalDeviceStateDir: cfg.Runtime.StateDir, LocalRuntime: runtimeAccess, CORSAllowedOrigins: cfg.Server.CorsAllowedOrigins, JWTSecret: tokens.SecretKey(), OnLocalCloudSession: func(summary *cloud.CloudSessionSummary) {
		if summary == nil {
			cloudConnector.Stop()
			return
		}
		cloudConnector.Start()
	}})
	return serveHTTP(serveCtx, cfg, logger, options, agentHandler, stdout, backendReady, errCh)
}

type cloudConnectorLifecycle struct {
	rootCtx       context.Context
	cfg           Config
	runtimeAccess agentapp.RuntimeAccess
	device        agentapp.Device
	logger        *slog.Logger
	options       Options
	stdout        io.Writer
	mu            sync.Mutex
	cancel        context.CancelFunc
	generation    uint64
}

func newCloudConnectorLifecycle(rootCtx context.Context, cfg Config, runtimeAccess agentapp.RuntimeAccess, device agentapp.Device, logger *slog.Logger, options Options, stdout io.Writer) *cloudConnectorLifecycle {
	return &cloudConnectorLifecycle{rootCtx: rootCtx, cfg: cfg, runtimeAccess: runtimeAccess, device: device, logger: logger, options: options, stdout: stdout}
}

func (c *cloudConnectorLifecycle) Start() {
	if !cloudConnectorConfigured(c.cfg) {
		return
	}
	c.mu.Lock()
	if c.cancel != nil {
		c.mu.Unlock()
		return
	}
	connectorCtx, cancel := context.WithCancel(c.rootCtx)
	c.cancel = cancel
	c.generation++
	generation := c.generation
	connector := newAgentConnector(c.cfg, c.cfg.Cloud.ApiBaseUrl, c.runtimeAccess, c.device, c.logger)
	c.mu.Unlock()

	_, _ = fmt.Fprintf(c.stdout, "TermBridge agent connector targeting %s\n", connector.Config().ConnectUrl)
	go c.run(connectorCtx, generation, connector)
}

func (c *cloudConnectorLifecycle) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *cloudConnectorLifecycle) run(ctx context.Context, generation uint64, connector *agentapp.Client) {
	defer func() {
		c.mu.Lock()
		if c.generation == generation {
			c.cancel = nil
		}
		c.mu.Unlock()
	}()
	for {
		err := normalizeServeError(runClient(c.options, ctx, connector))
		if err == nil {
			return
		}
		if c.logger != nil {
			c.logger.Warn("termbridge agent connector failed; agent dashboard remains available", "target", connector.Config().ConnectUrl, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func serveHTTP(ctx context.Context, cfg Config, logger *slog.Logger, options Options, agentHandler http.Handler, stdout io.Writer, backendReady chan struct{}, errCh chan error) error {
	if err := validateStaticDir(cfg.Server); err != nil {
		return apperrors.Runtime("validate static web directory", err)
	}
	server := httpserver.New(httpserver.Config{ServerUrl: cfg.Server.ListenURL}, backendHandler(cfg, logger, agentHandler))

	go func() {
		err := runBackendServer(options, ctx, server, func(info httpserver.Info) {
			_, _ = fmt.Fprintf(stdout, "TermBridge agent listening on %s\n", info.Url)
			close(backendReady)
		})
		errCh <- normalizeServeError(err)
	}()

	select {
	case <-backendReady:
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		return nil
	}

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		return nil
	}
}

func runBackendServer(options Options, ctx context.Context, server *httpserver.Server, onListening func(httpserver.Info)) error {
	if options.RunBackendServer != nil {
		return options.RunBackendServer(ctx, server, onListening)
	}
	listener, info, err := server.Listen()
	if err != nil {
		return err
	}
	if onListening != nil {
		onListening(info)
	}
	return server.Serve(ctx, listener)
}

func runClient(options Options, ctx context.Context, client *agentapp.Client) error {
	if options.RunClient != nil {
		return options.RunClient(ctx, client)
	}
	return client.Run(ctx)
}

func normalizeServeError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func newAgentConnector(cfg Config, connectURL string, runtimeAccess agentapp.RuntimeAccess, device agentapp.Device, logger *slog.Logger) *agentapp.Client {
	client := agentapp.New(agentapp.Config{ConnectUrl: connectURL, StateDir: cfg.Runtime.StateDir, Runtime: runtimeAccess, Logger: logger})
	client.SetDevice(device)
	return client
}

func cloudConnectorConfigured(cfg Config) bool {
	return cfg.Cloud.ApiBaseUrl != ""
}

func newWebTerminalRegistry(cfg Config, logger *slog.Logger, store state.DbStore) *terminalapp.Registry {
	return terminalapp.NewRegistry(terminalapp.Config{
		Cwd:              cfg.Cwd,
		Store:            store,
		ShortcutStore:    store,
		LogDir:           cfg.LogDir,
		History:          cfg.History,
		Manager:          gopty.NewManager(),
		Logger:           logger,
		ReplayMaxBytes:   cfg.Terminal.Replay.MaxBytes,
		ReplayChunkBytes: cfg.Terminal.Replay.ChunkBytes,
		ClientQueueSize:  cfg.Terminal.Client.Queue.MaxMessages,
		ClientQueueBytes: cfg.Terminal.Client.Queue.MaxBytes,
	})
}
