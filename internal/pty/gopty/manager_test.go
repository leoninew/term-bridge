//go:build windows

package gopty

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"termbridge-go/internal/process"
	termpty "termbridge-go/internal/pty"
)

func TestManagerRunsUnicodeAndAnsiOutput(t *testing.T) {
	output, result := runPowerShell(t, "Write-Output 'TERM_BRIDGE_UNICODE_BEGIN'; Write-Output '你好，TermBridge'; Write-Output \"`e[31mANSI_RED_TEXT`e[0m\"; Write-Output 'TERM_BRIDGE_UNICODE_END'")

	if result.ExitCode != 0 || result.Err != nil {
		t.Fatalf("Wait() = %#v", result)
	}
	for _, marker := range []string{"TERM_BRIDGE_UNICODE_BEGIN", "你好，TermBridge", "ANSI_RED_TEXT", "TERM_BRIDGE_UNICODE_END"} {
		if !strings.Contains(output, marker) {
			t.Fatalf("output missing %q:\n%s", marker, output)
		}
	}
}

func TestManagerPassesCwdAndEnv(t *testing.T) {
	cwd := t.TempDir()
	spec := powerShellSpec(t, "Write-Output ((Get-Location).Path); Write-Output ('TERM_BRIDGE_ENV_' + $env:TERMBRIDGE_GOPTY_TEST)")
	spec.Cwd = cwd
	spec.Env = append(spec.Env, "TERMBRIDGE_GOPTY_TEST=ok")

	output, result := runSpec(t, spec)

	if result.ExitCode != 0 || result.Err != nil {
		t.Fatalf("Wait() = %#v", result)
	}
	if !strings.Contains(output, cwd) {
		t.Fatalf("output missing cwd %q:\n%s", cwd, output)
	}
	if !strings.Contains(output, "TERM_BRIDGE_ENV_ok") {
		t.Fatalf("output missing env marker:\n%s", output)
	}
}

func TestManagerHandlesLongOutput(t *testing.T) {
	output, result := runPowerShell(t, "Write-Output 'TERM_BRIDGE_LONG_OUTPUT_BEGIN'; 1..1000 | ForEach-Object { Write-Output (\"TERM_BRIDGE_LONG_OUTPUT_LINE_{0:04d}\" -f $_) }; Write-Output 'TERM_BRIDGE_LONG_OUTPUT_END'")

	if result.ExitCode != 0 || result.Err != nil {
		t.Fatalf("Wait() = %#v", result)
	}
	if !strings.Contains(output, "TERM_BRIDGE_LONG_OUTPUT_BEGIN") || !strings.Contains(output, "TERM_BRIDGE_LONG_OUTPUT_END") {
		t.Fatalf("output missing long-output markers:\n%s", tail(output, 2000))
	}
}

func TestManagerReturnsUserExitCode(t *testing.T) {
	output, result := runPowerShell(t, "Write-Output 'TERM_BRIDGE_EXIT_CODE_7'; exit 7")

	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7; err=%v; output=%s", result.ExitCode, result.Err, output)
	}
	if !strings.Contains(output, "TERM_BRIDGE_EXIT_CODE_7") {
		t.Fatalf("output missing exit-code marker:\n%s", output)
	}
}

func TestManagerResizesSession(t *testing.T) {
	spec := powerShellSpec(t, "Start-Sleep -Milliseconds 300; Write-Output ('TERM_BRIDGE_SIZE_' + $Host.UI.RawUI.WindowSize.Width + 'x' + $Host.UI.RawUI.WindowSize.Height)")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, output, readDone, err := startSession(ctx, spec)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = session.Close()
		_ = session.KillTree()
	}()

	if err := session.Resize(process.TerminalSize{Cols: 100, Rows: 40}); err != nil {
		t.Fatalf("Resize() error = %v", err)
	}
	waitForOutput(t, output, "TERM_BRIDGE_SIZE_100x40", 5*time.Second)
	_ = session.Close()
	_ = waitForResult(t, session, 3*time.Second)
	waitForReader(t, readDone, output)
}

func runPowerShell(t *testing.T, script string) (string, termpty.Result) {
	t.Helper()
	spec := powerShellSpec(t, script)
	return runSpec(t, spec)
}

func powerShellSpec(t *testing.T, script string) process.ProcessSpec {
	t.Helper()
	return process.ProcessSpec{
		Command:     resolvePowerShell(t),
		Args:        []string{"-NoLogo", "-NoProfile", "-Command", script},
		Cwd:         t.TempDir(),
		Env:         append(os.Environ(), "TERMBRIDGE_GOPTY_TEST=ok"),
		InitialSize: process.TerminalSize{Cols: 80, Rows: 25},
	}
}

func runSpec(t *testing.T, spec process.ProcessSpec) (string, termpty.Result) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, output, readDone, err := startSession(ctx, spec)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = session.Close()
		_ = session.KillTree()
	}()

	result := waitForResult(t, session, 10*time.Second)
	if err := session.Close(); err != nil {
		_ = session.KillTree()
	}
	waitForReader(t, readDone, output)
	return output.String(), result
}

func startSession(ctx context.Context, spec process.ProcessSpec) (termpty.Session, *bytes.Buffer, chan error, error) {
	session, err := NewManager().Start(ctx, spec)
	if err != nil {
		return nil, nil, nil, err
	}

	var output bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(&output, session)
		readDone <- err
	}()

	return session, &output, readDone, nil
}

func waitForResult(t *testing.T, session termpty.Session, timeout time.Duration) termpty.Result {
	t.Helper()
	waitCh := make(chan termpty.Result, 1)
	go func() { waitCh <- session.Wait() }()

	select {
	case result := <-waitCh:
		return result
	case <-time.After(timeout):
		_ = session.Close()
		_ = session.KillTree()
		t.Fatal("timed out waiting for PTY session")
		return termpty.Result{}
	}
}

func waitForOutput(t *testing.T, output *bytes.Buffer, substr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(output.String(), substr) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q; output:\n%s", substr, output.String())
}

func waitForReader(t *testing.T, readDone chan error, output *bytes.Buffer) {
	t.Helper()
	select {
	case err := <-readDone:
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") && err != io.EOF {
			t.Fatalf("Read() error = %v; output=%s", err, output.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for PTY output reader; output=%s", output.String())
	}
}

func resolvePowerShell(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	t.Skip("no PowerShell executable found")
	return ""
}

func tail(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}
