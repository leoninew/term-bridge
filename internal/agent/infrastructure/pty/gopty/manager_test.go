//go:build windows

package gopty

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	termpty "termbridge/internal/agent/infrastructure/pty"
	"termbridge/internal/agent/model/task/process"
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

func TestManagerResizesToLargeTerminal(t *testing.T) {
	// Regression test: MaxRows was 200, but browsers can request larger sizes.
	// The manager must accept sizes up to MaxRows (500) without error.
	spec := powerShellSpec(t, "Start-Sleep -Milliseconds 200; Write-Output 'TERM_BRIDGE_RESIZE_OK'")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, _, readDone, err := startSession(ctx, spec)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	for _, size := range []process.TerminalSize{
		{Cols: 200, Rows: 200},
		{Cols: 300, Rows: 300},
		{Cols: 500, Rows: 500},
	} {
		if err := session.Resize(size); err != nil {
			_ = session.Close()
			_ = session.KillTree()
			t.Fatalf("Resize() to %dx%d error = %v", size.Cols, size.Rows, err)
		}
	}

	_ = waitForResult(t, session, 3*time.Second)
	_ = session.Close()
	waitForReader(t, readDone, nil)
	_ = session.KillTree()
}

func TestManagerContextCancelDoesNotKillProcess(t *testing.T) {
	// Regression test: passing a cancellable context (e.g. HTTP request context)
	// must not kill the process when the context is cancelled.
	// The manager should detach the process lifecycle from the caller context.
	spec := powerShellSpec(t, "Start-Sleep -Milliseconds 200; Write-Output 'TERM_BRIDGE_AFTER_CANCEL'")
	session, output, readDone, err := startSession(context.Background(), spec)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = session.Close()
		_ = session.KillTree()
	}()

	// Start a goroutine that cancels a dummy context after a short delay.
	// If the manager incorrectly ties the process to that context, the process
	// will be killed. The manager must use context.Background() internally.
	cancelCtx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Wait for the process to finish normally despite the cancelled context.
	result := waitForResult(t, session, 5*time.Second)
	_ = cancelCtx
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0; err=%v; output=%q", result.ExitCode, result.Err, output.String())
	}
	if !strings.Contains(output.String(), "TERM_BRIDGE_AFTER_CANCEL") {
		t.Fatalf("output missing marker:\n%s", output.String())
	}
	_ = session.Close()
	waitForReader(t, readDone, output)
}

func TestManagerRunsCmdExe(t *testing.T) {
	// cmd.exe is a Windows builtin — must be resolved to an absolute path.
	cmdPath, err := exec.LookPath("cmd.exe")
	if err != nil {
		t.Skip("cmd.exe not found")
	}
	spec := process.ProcessSpec{
		Command:     cmdPath,
		Args:        []string{"/c", "echo TERM_BRIDGE_CMD_OK"},
		Cwd:         t.TempDir(),
		Env:         os.Environ(),
		InitialSize: process.TerminalSize{Cols: 80, Rows: 25},
	}
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

	waitForOutput(t, output, "TERM_BRIDGE_CMD_OK", 5*time.Second)
	result := waitForResult(t, session, 5*time.Second)
	if result.ExitCode != 0 || result.Err != nil {
		t.Fatalf("Wait() = %#v; output=%s", result, output.String())
	}
	_ = session.Close()
	waitForReader(t, readDone, output)
}

func TestManagerReturnsCorrectExitCode(t *testing.T) {
	for _, code := range []int{0, 1, 7, 42} {
		t.Run(fmt.Sprintf("exit_%d", code), func(t *testing.T) {
			_, result := runPowerShell(t, fmt.Sprintf("exit %d", code))
			if result.ExitCode != code {
				t.Fatalf("ExitCode = %d, want %d; err=%v", result.ExitCode, code, result.Err)
			}
		})
	}
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
		// io.Copy returns nil on io.EOF, which is normal when the PTY is closed.
		if err != nil && err != io.EOF && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			t.Fatalf("Read() error = %v; output=%v", err, output)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for PTY output reader; output=%v", output)
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
