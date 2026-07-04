package runner

import (
	"context"
	"errors"
	"io"

	termpty "termbridge-go/internal/agent/infrastructure/pty"
)

func relayInput(ctx context.Context, stdin io.Reader, session termpty.Session, ctrlC chan<- struct{}) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := stdin.Read(buf)
		if n > 0 {
			writeInputChunk(session, buf[:n], ctrlC)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				_ = session.Close()
			}
			return
		}
	}
}

func writeInputChunk(session termpty.Session, chunk []byte, ctrlC chan<- struct{}) {
	start := 0
	for i, b := range chunk {
		if b != 3 {
			continue
		}
		if start < i {
			_, _ = session.Write(chunk[start:i])
		}
		select {
		case ctrlC <- struct{}{}:
		default:
		}
		start = i + 1
	}
	if start < len(chunk) {
		_, _ = session.Write(chunk[start:])
	}
}
