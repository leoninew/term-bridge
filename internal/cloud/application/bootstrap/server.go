package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	cloudapi "gitee.com/leoninew/TermBridge-go/internal/cloud/api/handler"
	cloudauth "gitee.com/leoninew/TermBridge-go/internal/cloud/application/user/auth"
	clouddb "gitee.com/leoninew/TermBridge-go/internal/cloud/infrastructure/database"
	cloudemail "gitee.com/leoninew/TermBridge-go/internal/cloud/infrastructure/email"
	cloudauthrepo "gitee.com/leoninew/TermBridge-go/internal/cloud/repository/user/auth"
	clouddevice "gitee.com/leoninew/TermBridge-go/internal/cloud/repository/user/device"
	httpserver "gitee.com/leoninew/TermBridge-go/internal/shared/api/server"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	basedb "gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/database"
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
	db, err := basedb.Open(ctx, cfg.Database.Driver, cfg.Database.SQLite.Path, cfg.Database.MySQL.Dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := clouddb.Migrate(ctx, db.DB, db.Driver); err != nil {
		return err
	}

	deviceRepository := clouddevice.NewRepository(db.DB, db.Driver)
	repo := cloudauthrepo.New(db.DB, db.Driver)
	tokens, err := sharedauth.NewTokenServiceFromBase64Key(cfg.Jwt.SecretKey, cfg.Auth.JwtTTL)
	if err != nil {
		return apperrors.Config("invalid jwt.secret_key", err)
	}
	expectedHostname, err := validateTurnstileConfig(cfg.Environment, cfg.Cloud)
	if err != nil {
		return apperrors.Config("invalid cloud.turnstile", err)
	}
	authService := cloudauth.New(repo, tokens, cloudauth.Config{PasswordPolicy: cloudauth.PasswordPolicy{MinLength: cfg.Auth.PasswordPolicy.MinLength, MaxLength: cfg.Auth.PasswordPolicy.MaxLength}, Code: cloudauth.CodePolicy{Length: cfg.Auth.Code.Length, Ttl: cfg.Auth.Code.Ttl, ResendCooldown: cfg.Auth.Code.ResendCooldown, MaxAttempts: cfg.Auth.Code.MaxAttempts}}, cloudemail.NewResendSender(cloudemail.Config{ApiKey: cfg.Resend.ApiKey, FromEmail: cfg.Resend.FromEmail}), cloudauth.NewOAuthGoogleClient(cloudauth.GoogleConfig{ClientID: cfg.Auth.Google.ClientID, ClientSecret: cfg.Auth.Google.ClientSecret, RedirectUrl: cfg.Auth.Google.RedirectUrl}))
	cloudHandler := cloudapi.New(cloudapi.Config{DebugErrors: cfg.Gate.API.ExposeErrors, Logger: logger, AuthService: authService, AgentTunnelAudience: tunnelAudience(cfg), DeviceRepository: cloudapi.NewDeviceRepository(deviceRepository), CloudPublicURL: cfg.Cloud.PublicURL, CloudOAuth: cloudapi.CloudOAuthConfig{Clients: cloudOAuthClients(cfg.Cloud.OAuth.Clients)}, CORSAllowedOrigins: cfg.Server.CorsAllowedOrigins, JWTSecret: tokens.SecretKey(), Turnstile: cloudapi.TurnstileConfig{SiteKey: cfg.Cloud.Turnstile.SiteKey, SecretKey: cfg.Cloud.Turnstile.SecretKey, ExpectedHostname: expectedHostname, Verify: cloudapi.NewTurnstileVerifier(cfg.Cloud.Turnstile.SecretKey, expectedHostname, nil)}, CSRF: cloudapi.CSRFConfig{Tokens: cloudapi.NewCSRFTokens(10*time.Minute, 1024)}})
	return serveHTTP(ctx, cfg, logger, options, cloudHandler, stdout)
}

func validateTurnstileConfig(environment string, cfg CloudConfig) (string, error) {
	if environment != "production" {
		return "", nil
	}
	if strings.TrimSpace(cfg.Turnstile.SiteKey) == "" || strings.TrimSpace(cfg.Turnstile.SecretKey) == "" {
		return "", fmt.Errorf("site key and secret key are required")
	}
	parsed, err := url.Parse(cfg.PublicURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return "", fmt.Errorf("public URL must be an absolute HTTPS URL with a hostname")
	}
	return parsed.Hostname(), nil
}

func cloudOAuthClients(clients []CloudOAuthClientConfig) []cloudapi.CloudOAuthClientConfig {
	out := make([]cloudapi.CloudOAuthClientConfig, 0, len(clients))
	for _, client := range clients {
		out = append(out, cloudapi.CloudOAuthClientConfig{
			ClientId:     client.ClientId,
			ClientSecret: client.ClientSecret,
			RedirectUrl:  client.RedirectUrl,
			Scopes:       append([]string(nil), client.Scopes...),
		})
	}
	return out
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
	if strings.TrimSpace(cfg.Cloud.PublicURL) != "" {
		return cfg.Cloud.PublicURL
	}
	return cfg.Server.ListenURL
}
