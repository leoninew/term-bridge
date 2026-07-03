package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentapi "termbridge-go/internal/agent/api"
	agentapp "termbridge-go/internal/agent/application"
	terminalapp "termbridge-go/internal/agent/application/terminal"
	agentdevice "termbridge-go/internal/agent/device"
	agentdb "termbridge-go/internal/agent/infrastructure/database"
	"termbridge-go/internal/agent/pty/gopty"
	"termbridge-go/internal/agent/state"
	sharedauth "termbridge-go/internal/shared/auth"
	basedb "termbridge-go/internal/shared/database"
	apperrors "termbridge-go/internal/shared/errors"
	httpserver "termbridge-go/internal/shared/http/server"
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
	db, err := basedb.Open(ctx, cfg.Database.Driver, cfg.Database.SQLite.Path, cfg.Database.MySQL.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := agentdb.Migrate(ctx, db.DB, db.Driver); err != nil {
		return err
	}

	deviceRepository := agentdevice.NewRepository(db.DB, db.Driver)
	tokens, err := sharedauth.NewTokenServiceFromBase64Key(cfg.JWT.SecretKey, cfg.Auth.JWTTTL)
	if err != nil {
		return apperrors.Config("invalid jwt.secret_key", err)
	}
	authService := agentapi.NewLocalAuthService(agentapi.LocalAuthConfig{Username: cfg.Auth.Username, Password: cfg.Auth.Password}, tokens)

	device, err := agentapp.LoadOrCreateDevice(agentapp.DeviceOptions{StateDir: cfg.Runtime.StateDir})
	if err != nil {
		return err
	}
	if _, err := deviceRepository.UpsertLocalDevice(ctx, agentdevice.Device{ID: device.Id, Name: device.Name, PublicKey: device.PublicKey}); err != nil {
		return apperrors.Runtime("upsert local device", err)
	}
	runtimeAccess := agentapp.WebTerminalAccess{Registry: newWebTerminalRegistry(cfg, logger, state.NewDBStore(db.DB, db.Driver, filepath.Join(cfg.Runtime.StateDir, "devices", device.Id), device.Id))}

	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	backendReady := make(chan struct{})
	errCh := make(chan error, 3)
	var connectorMu sync.Mutex
	cloudConnectorStarted := false
	var cloudConnector *agentapp.Client
	if cloudConnectorConfigured(cfg) {
		cloudConnector = newAgentConnector(cfg, cfg.Cloud.GateURL, runtimeAccess, device, logger)
	}
	startConnector := func(connector *agentapp.Client) {
		connectorMu.Lock()
		if connector == cloudConnector {
			if cloudConnectorStarted {
				connectorMu.Unlock()
				return
			}
			cloudConnectorStarted = true
		}
		connectorMu.Unlock()
		fmt.Fprintf(stdout, "TermBridge agent connector targeting %s\n", connector.Config().ConnectUrl)
		go func() {
			for {
				err := normalizeServeError(runClient(options, serveCtx, connector))
				if err == nil {
					errCh <- nil
					return
				}
				logger.Warn("termbridge agent connector failed; local dashboard remains available", "target", connector.Config().ConnectUrl, "error", err)
				select {
				case <-serveCtx.Done():
					errCh <- nil
					return
				case <-time.After(2 * time.Second):
				}
			}
		}()
	}

	agentHandler := agentapi.New(agentapi.Config{DebugErrors: cfg.Gate.API.ExposeErrors, Logger: logger, AuthService: authService, CloudGateURL: cfg.Cloud.GateURL, CloudOAuth: agentapi.CloudOAuthConfig{ClientID: cfg.Cloud.OAuth.ClientID, ClientSecret: cfg.Cloud.OAuth.ClientSecret, RedirectURL: cfg.Cloud.OAuth.RedirectURL, Scopes: cfg.Cloud.OAuth.Scopes}, CloudOAuthAttemptStore: agentapi.NewCloudOAuthAttemptStore(cfg.Runtime.StateDir), LocalDevice: device, LocalRuntime: runtimeAccess, CORSAllowedOrigins: cfg.Server.CORSAllowedOrigins, JWTSecret: tokens.SecretKey(), OnLocalCloudSession: func(agentapi.CloudSessionSummary) {
		if cloudConnector != nil {
			startConnector(cloudConnector)
		}
	}})
	return serveHTTP(serveCtx, cfg, logger, options, agentHandler, stdout, backendReady, errCh)
}

func serveHTTP(ctx context.Context, cfg Config, logger *slog.Logger, options Options, agentHandler http.Handler, stdout io.Writer, backendReady chan struct{}, errCh chan error) error {
	if err := validateStaticDir(cfg.Server.StaticDir); err != nil {
		return apperrors.Runtime("validate static web directory", err)
	}
	server := httpserver.New(httpserver.Config{ServerUrl: cfg.Server.ListenURL}, backendHandler(cfg, logger, agentHandler))

	go func() {
		err := runBackendServer(options, ctx, server, func(info httpserver.Info) {
			fmt.Fprintf(stdout, "TermBridge agent listening on %s\n", info.Url)
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
	client := agentapp.New(agentapp.Config{ConnectUrl: connectURL, Username: cfg.Auth.Username, Password: cfg.Auth.Password, StateDir: cfg.Runtime.StateDir, Runtime: runtimeAccess, Logger: logger})
	client.SetDevice(device)
	return client
}

func cloudConnectorConfigured(cfg Config) bool {
	return strings.TrimSpace(cfg.Cloud.GateURL) != ""
}

func newWebTerminalRegistry(cfg Config, logger *slog.Logger, store terminalapp.RuntimeStore) *terminalapp.Registry {
	return terminalapp.NewRegistry(terminalapp.Config{
		Cwd:     cfg.Cwd,
		Store:   store,
		LogDir:  cfg.LogDir,
		History: cfg.History,
		Manager: gopty.NewManager(),
		Logger:  logger,
	})
}
