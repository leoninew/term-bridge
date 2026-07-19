package app

import (
	"time"

	browserdto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/browser"
	sharedconfig "gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config"
)

type Config struct {
	Environment string
	LogHTTP     sharedconfig.LogHTTPConfig
	Server      ServerConfig
	Gate        GateConfig
	Database    DatabaseConfig
	Cloud       CloudConfig
	Terminal    TerminalConfig
}

type TerminalConfig struct {
	Quota TerminalQuotaConfig
}

type TerminalQuotaConfig struct {
	ConcurrentAttaches int
}

type ServerConfig struct {
	ListenURL            string
	StaticDir            string
	BrowserRuntimeConfig browserdto.RuntimeConfig
	CorsAllowedOrigins   []string
}

type GateConfig struct {
	API GateApiConfig
}

type GateApiConfig struct {
	ExposeErrors bool
}

type DatabaseConfig struct {
	Driver string
	SQLite SQLiteConfig
	MySQL  MySQLConfig
}

type SQLiteConfig struct {
	Path string
}

type MySQLConfig struct {
	Dsn string
}

type JwtConfig struct {
	SecretKey string
	Ttl       time.Duration
}

type GoogleConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
}

type GitHubConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
}

type ResendConfig struct {
	ApiKey    string
	FromEmail string
}

type CloudConfig struct {
	PublicURL  string
	ApiBaseUrl string
	Admin      CloudAdminConfig
	Jwt        JwtConfig
	Google     GoogleConfig
	GitHub     GitHubConfig
	Resend     ResendConfig
	Turnstile  TurnstileConfig
	OAuth      CloudOAuthConfig
}

type CloudAdminConfig struct {
	UserIds []string
	Emails  []string
}

type TurnstileConfig struct {
	SiteKey   string
	SecretKey string
}

type CloudOAuthConfig struct {
	Clients []CloudOAuthClientConfig
}

type CloudOAuthClientConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
	Scopes       []string
}
