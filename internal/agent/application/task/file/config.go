package file

import (
	"fmt"
	"time"
)

type Config struct {
	MaxTextBytes              int64
	MaxDirectoryEntries       int
	MaxRecursiveDeleteEntries int
	OperationTimeout          time.Duration
}

func (c Config) Validate() error {
	if c.MaxTextBytes < 1 {
		return fmt.Errorf("max text bytes must be positive")
	}
	if c.MaxDirectoryEntries < 1 {
		return fmt.Errorf("max directory entries must be positive")
	}
	if c.MaxRecursiveDeleteEntries < 1 {
		return fmt.Errorf("max recursive delete entries must be positive")
	}
	if c.OperationTimeout <= 0 {
		return fmt.Errorf("operation timeout must be positive")
	}
	return nil
}
