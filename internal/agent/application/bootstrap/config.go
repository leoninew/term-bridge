package app

import (
	"time"

	"gitee.com/leoninew/TermBridge-go/internal/agent/infrastructure/storage/history"
	browserdto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/browser"
	sharedconfig "gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config"
)

type Config struct {
	Cwd      string
	Command  []string
	LogDir   string
	LogHTTP  sharedconfig.LogHTTPConfig
	History  history.Config
	Terminal TerminalConfig
	File     FileConfig
	Git      GitConfig
	Runtime  RuntimeConfig
	Server   ServerConfig
	Gate     GateConfig
	Database DatabaseConfig
	Cloud    CloudConnectorConfig
}

type TerminalConfig struct {
	Replay TerminalReplayConfig
	Client TerminalClientConfig
}

type TerminalReplayConfig struct {
	MaxBytes   int64
	ChunkBytes int
}

type TerminalClientConfig struct {
	Queue TerminalClientQueueConfig
}

type TerminalClientQueueConfig struct {
	MaxMessages int
	MaxBytes    int
}

type FileConfig struct {
	MaxTextBytes              int64
	MaxDirectoryEntries       int
	MaxRecursiveDeleteEntries int
	OperationTimeout          time.Duration
	WatchSubscriberQueueSize  int
}

type GitConfig struct {
	Executable     string
	CommandTimeout time.Duration
	MaxStdoutBytes int64
	MaxStderrBytes int64
	MaxTextBytes   int64
}

type RuntimeConfig struct {
	StateDir string
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

type CloudConnectorConfig struct {
	PublicURL   string
	ApiBaseUrl  string
	OAuthClient OAuthClientConfig
}

type OAuthClientConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
	Scopes       []string
}
