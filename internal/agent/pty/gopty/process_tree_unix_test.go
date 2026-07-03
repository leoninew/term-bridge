//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package gopty

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"termbridge-go/internal/agent/domain/process"
)

func TestKillTreeStopsUnixChildProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix process group test")
	}
	marker := filepath.Join(t.TempDir(), "child.pid")
	script := "sleep 30 & echo $! > " + shellQuote(marker) + "; echo TERM_BRIDGE_CHILD_READY; wait"
	spec := process.ProcessSpec{Command: "/bin/sh", Args: []string{"-c", script}, Cwd: t.TempDir(), Env: os.Environ(), InitialSize: process.DefaultTerminalSize()}
	session, err := NewManager().Start(context.Background(), spec)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	var output bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(&output, session)
		readDone <- err
	}()
	waitForFile(t, marker)
	childPid := strings.TrimSpace(string(readBytes(t, marker)))
	if childPid == "" {
		t.Fatal("child pid marker is empty")
	}
	if err := session.KillTree(); err != nil {
		t.Fatalf("KillTree() error = %v", err)
	}
	waitForProcessExit(t, childPid)
	_ = session.Close()
	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("reader did not finish; output=%s", output.String())
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func waitForProcessExit(t *testing.T, pid string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		cmd := exec.Command("kill", "-0", pid)
		if err := cmd.Run(); err != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("process %s is still alive", pid)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
