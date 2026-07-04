package app

import (
	"time"

	"termbridge-go/internal/agent/infrastructure/storage/history"
)

type Config struct {
	Cwd      string
	Command  []string
	LogDir   string
	LogHTTP  LogHTTPConfig
	History  history.Config
	Runtime  RuntimeConfig
	Server   ServerConfig
	Gate     GateConfig
	Database DatabaseConfig
	Auth     AuthConfig
	JWT      JWTConfig
	Cloud    CloudConnectorConfig
}

type LogHTTPConfig struct {
	RequestBodyLimit  int
	ResponseBodyLimit int
}

type RuntimeConfig struct {
	StateDir string
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
	Username string
	Password string
	JWTTTL   time.Duration
}

type JWTConfig struct {
	SecretKey string
}

type CloudConnectorConfig struct {
	GateURL string
	OAuth   CloudOAuthConfig
}

type CloudOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}
