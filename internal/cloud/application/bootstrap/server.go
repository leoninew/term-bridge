package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	cloudapi "termbridge-go/internal/cloud/api/handler"
	cloudauth "termbridge-go/internal/cloud/application/user/auth"
	clouddb "termbridge-go/internal/cloud/infrastructure/database"
	cloudemail "termbridge-go/internal/cloud/infrastructure/email"
	cloudauthrepo "termbridge-go/internal/cloud/repository/user/auth"
	clouddevice "termbridge-go/internal/cloud/repository/user/device"
	httpserver "termbridge-go/internal/shared/api/server"
	sharedauth "termbridge-go/internal/shared/common/auth"
	apperrors "termbridge-go/internal/shared/common/errors"
	basedb "termbridge-go/internal/shared/infrastructure/database"
)

type Options struct {
	Stdout           io.Writer
	RunBackendServer func(context.Context, *httpserver.Server, func(httpserver.Info)) error
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
	defer func() { _ = db.Close() }()
	if err := clouddb.Migrate(ctx, db.DB, db.Driver); err != nil {
		return err
	}

	deviceRepository := clouddevice.NewRepository(db.DB, db.Driver)
	repo := cloudauthrepo.New(db.DB, db.Driver)
	tokens, err := sharedauth.NewTokenServiceFromBase64Key(cfg.JWT.SecretKey, cfg.Auth.JWTTTL)
	if err != nil {
		return apperrors.Config("invalid jwt.secret_key", err)
	}
	authService := cloudauth.New(repo, tokens, cloudauth.Config{PasswordPolicy: cloudauth.PasswordPolicy{MinLength: cfg.Auth.PasswordPolicy.MinLength, MaxLength: cfg.Auth.PasswordPolicy.MaxLength}, Code: cloudauth.CodePolicy{Length: cfg.Auth.Code.Length, TTL: cfg.Auth.Code.TTL, ResendCooldown: cfg.Auth.Code.ResendCooldown, MaxAttempts: cfg.Auth.Code.MaxAttempts}}, cloudemail.NewResendSender(cloudemail.Config{APIKey: cfg.Resend.APIKey, FromEmail: cfg.Resend.FromEmail}), cloudauth.NewOAuthGoogleClient(cloudauth.GoogleConfig{ClientID: cfg.Auth.Google.ClientID, ClientSecret: cfg.Auth.Google.ClientSecret, RedirectURL: cfg.Auth.Google.RedirectURL}))
	cloudHandler := cloudapi.New(cloudapi.Config{DebugErrors: cfg.Gate.API.ExposeErrors, Logger: logger, AuthService: authService, AgentTunnelAudience: tunnelAudience(cfg), DeviceRepository: cloudapi.NewDeviceRepository(deviceRepository), CloudGateURL: cfg.Cloud.GateURL, CloudOAuth: cloudapi.CloudOAuthConfig{ClientID: cfg.Cloud.OAuth.ClientID, ClientSecret: cfg.Cloud.OAuth.ClientSecret, RedirectURL: cfg.Cloud.OAuth.RedirectURL, Scopes: cfg.Cloud.OAuth.Scopes}, CORSAllowedOrigins: cfg.Server.CORSAllowedOrigins, JWTSecret: tokens.SecretKey()})
	return serveHTTP(ctx, cfg, logger, options, cloudHandler, stdout)
}

func serveHTTP(ctx context.Context, cfg Config, logger *slog.Logger, options Options, cloudHandler http.Handler, stdout io.Writer) error {
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	backendReady := make(chan struct{})
	errCh := make(chan error, 1)
	if err := validateStaticDir(cfg.Server.StaticDir); err != nil {
		return apperrors.Runtime("validate static web directory", err)
	}
	server := httpserver.New(httpserver.Config{ServerUrl: cfg.Server.ListenURL}, backendHandler(cfg, logger, cloudHandler))

	go func() {
		err := runBackendServer(options, serveCtx, server, func(info httpserver.Info) {
			_, _ = fmt.Fprintf(stdout, "TermBridge cloud listening on %s\n", info.Url)
			close(backendReady)
		})
		errCh <- normalizeServeError(err)
	}()

	select {
	case <-backendReady:
	case err := <-errCh:
		cancel()
		if err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		cancel()
		return nil
	}

	select {
	case err := <-errCh:
		cancel()
		if err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		cancel()
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

func normalizeServeError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func tunnelAudience(cfg Config) string {
	if strings.TrimSpace(cfg.Cloud.GateURL) != "" {
		return cfg.Cloud.GateURL
	}
	return cfg.Server.ListenURL
}
