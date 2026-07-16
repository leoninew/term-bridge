package git

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	Executable     string
	CommandTimeout time.Duration
	MaxStdoutBytes int64
	MaxStderrBytes int64
	MaxTextBytes   int64
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Executable) == "" {
		return fmt.Errorf("git executable is required")
	}
	if c.CommandTimeout <= 0 {
		return fmt.Errorf("git command timeout must be positive")
	}
	if c.MaxStdoutBytes < 1 || c.MaxStderrBytes < 1 || c.MaxTextBytes < 1 {
		return fmt.Errorf("git output limits must be positive")
	}
	if c.MaxTextBytes > c.MaxStdoutBytes {
		return fmt.Errorf("git text limit must not exceed stdout limit")
	}
	return nil
}
