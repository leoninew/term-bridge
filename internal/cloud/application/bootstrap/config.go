package app

import "time"

type Config struct {
	LogHTTP  LogHTTPConfig
	Server   ServerConfig
	Gate     GateConfig
	Database DatabaseConfig
	Auth     AuthConfig
	JWT      JWTConfig
	Resend   ResendConfig
	Cloud    CloudConfig
}

type LogHTTPConfig struct {
	RequestBodyLimit  int
	ResponseBodyLimit int
}

type ServerConfig struct {
	ListenURL          string
	StaticDir          string
	APIBaseURL         string
	CORSAllowedOrigins []string
}

type GateConfig struct {
	API GateAPIConfig
}

type GateAPIConfig struct {
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
	DSN string
}

type AuthConfig struct {
	JWTTTL         time.Duration
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
	TTL            time.Duration
	ResendCooldown time.Duration
	MaxAttempts    int
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type JWTConfig struct {
	SecretKey string
}

type ResendConfig struct {
	APIKey    string
	FromEmail string
}

type CloudConfig struct {
	GateURL string
	OAuth   CloudOAuthConfig
}

type CloudOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}
