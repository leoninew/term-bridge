package app

import (
	"time"

	"gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/storage/history"
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
	Jwt      JwtConfig
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
	JwtTTL time.Duration
}

type JwtConfig struct {
	SecretKey string
}

type CloudConnectorConfig struct {
	PublicURL   string
	OAuthClient OAuthClientConfig
}

type OAuthClientConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
	Scopes       []string
}
