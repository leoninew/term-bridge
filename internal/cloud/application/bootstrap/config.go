package app

import (
	"time"

	sharedconfig "gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config"
)

type Config struct {
	Environment string
	LogHTTP     sharedconfig.LogHTTPConfig
	Server      ServerConfig
	Gate        GateConfig
	Database    DatabaseConfig
	Auth        AuthConfig
	Jwt         JwtConfig
	Resend      ResendConfig
	Cloud       CloudConfig
}

type ServerConfig struct {
	ListenURL          string
	StaticDir          string
	ApiBaseUrl         string
	CorsAllowedOrigins []string
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

type AuthConfig struct {
	JwtTTL         time.Duration
	PasswordPolicy PasswordPolicy
	Code           CodePolicy
	Google         GoogleConfig
}

type PasswordPolicy struct {
	MinLength int
	MaxLength int
}

type CodePolicy struct {
	Length         int
	Ttl            time.Duration
	ResendCooldown time.Duration
	MaxAttempts    int
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectUrl  string
}

type JwtConfig struct {
	SecretKey string
}

type ResendConfig struct {
	ApiKey    string
	FromEmail string
}

type CloudConfig struct {
	PublicURL string
	Turnstile TurnstileConfig
	OAuth     CloudOAuthConfig
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
