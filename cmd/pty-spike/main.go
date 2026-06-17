package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

type result struct {
	Name     string
	Passed   bool
	Duration time.Duration
	Error    string
}

type session struct {
	pty    pty.Pty
	cmd    *pty.Cmd
	buf    bytes.Buffer
	mu     sync.Mutex
	done   chan error
	closed chan struct{}
	subs   map[chan []byte]struct{}
}

func main() {
	internalOwnerCrashChild := flag.Bool("internal-owner-crash-child", false, "run owner crash child scenario")
	internalMarker := flag.String("internal-marker", "", "internal process marker")
	scenario := flag.String("scenario", "all", "scenario: all, basic, cwd, env, unicode, long-output, resize, cleanup, agents, interrupt, concurrent, multi-attach, owner-crash, descendant-cleanup, focused")
	shell := flag.String("shell", "all", "shell: all, cmd, powershell, pwsh")
	timeout := flag.Duration("timeout", 10*time.Second, "per-step timeout")
	flag.Parse()

	if *internalOwnerCrashChild {
		runOwnerCrashChild(*internalMarker, *timeout)
		return
	}

	started := time.Now()
	results := run(*scenario, *shell, *timeout)
	failed := 0
	for _, item := range results {
		status := "PASS"
		if !item.Passed {
			status = "FAIL"
			failed++
		}
		fmt.Printf("[%s] %s (%s)\n", status, item.Name, item.Duration.Round(time.Millisecond))
		if item.Error != "" {
			fmt.Printf("       %s\n", item.Error)
		}
	}
	fmt.Printf("\nsummary: total=%d failed=%d duration=%s\n", len(results), failed, time.Since(started).Round(time.Millisecond))
	if failed > 0 {
		os.Exit(1)
	}
}

func run(scenario string, shell string, timeout time.Duration) []result {
	var results []result
	shells := selectedShells(shell)
	if scenario == "all" || scenario == "basic" {
		for _, sh := range shells {
			results = append(results, runShellBasic(sh, timeout))
		}
	}
	if scenario == "all" || scenario == "cwd" {
		for _, sh := range shells {
			results = append(results, runShellCwd(sh, timeout))
		}
	}
	if scenario == "all" || scenario == "env" {
		for _, sh := range shells {
			results = append(results, runShellEnv(sh, timeout))
		}
	}
	if scenario == "all" || scenario == "resize" {
		for _, sh := range shells {
			results = append(results, runShellResize(sh, timeout))
		}
	}
	if scenario == "all" || scenario == "unicode" {
		results = append(results, runTestProgram("unicode", []string{"--mode", "unicode"}, timeout))
	}
	if scenario == "all" || scenario == "long-output" {
		results = append(results, runTestProgram("long-output", []string{"--mode", "long-output", "--lines", "20000"}, 30*time.Second))
	}
	if scenario == "all" || scenario == "cleanup" {
		for _, sh := range shells {
			results = append(results, runCleanup(sh, timeout))
		}
	}
	if scenario == "all" || scenario == "agents" {
		results = append(results, runAgentVersion("claude", []string{"--version"}, "Claude Code", timeout))
		results = append(results, runAgentVersion("codex", []string{"--version"}, "codex-cli", timeout))
	}
	if scenario == "all" || scenario == "focused" || scenario == "interrupt" {
		for _, sh := range shells {
			results = append(results, runInterrupt(sh, timeout))
		}
	}
	if scenario == "all" || scenario == "focused" || scenario == "concurrent" {
		results = append(results, runConcurrent(timeout))
	}
	if scenario == "all" || scenario == "focused" || scenario == "multi-attach" {
		for _, sh := range shells {
			results = append(results, runMultiAttach(sh, timeout))
		}
	}
	if scenario == "all" || scenario == "focused" || scenario == "owner-crash" {
		results = append(results, runOwnerCrash(timeout))
	}
	if scenario == "all" || scenario == "focused" || scenario == "descendant-cleanup" {
		for _, sh := range shells {
			results = append(results, runDescendantCleanup(sh, timeout))
		}
	}
	return results
}

func selectedShells(shell string) []string {
	if shell == "all" {
		return []string{"cmd", "powershell", "pwsh"}
	}
	return []string{shell}
}

func runShellBasic(shell string, timeout time.Duration) result {
	return measure("basic "+shell, func() error {
		s, err := startShell(shell, "", nil)
		if err != nil {
			return err
		}
		defer s.close()
		if err := s.writeLine(shell, echoCommand(shell, "TERM_BRIDGE_BASIC_OK")); err != nil {
			return err
		}
		if err := s.waitFor("TERM_BRIDGE_BASIC_OK", timeout); err != nil {
			return err
		}
		if err := s.writeLine(shell, "exit"); err != nil {
			return err
		}
		return s.waitExit(timeout)
	})
}

func runShellCwd(shell string, timeout time.Duration) result {
	cwd := filepath.Clean(`D:\SourceCodes\mywork`)
	return measure("cwd "+shell, func() error {
		s, err := startShell(shell, cwd, nil)
		if err != nil {
			return err
		}
		defer s.close()
		if err := s.writeLine(shell, cwdCommand(shell)); err != nil {
			return err
		}
		if err := s.waitFor("D:\\SourceCodes\\mywork", timeout); err != nil {
			return err
		}
		if err := s.writeLine(shell, "exit"); err != nil {
			return err
		}
		return s.waitExit(timeout)
	})
}

func runShellEnv(shell string, timeout time.Duration) result {
	return measure("env "+shell, func() error {
		s, err := startShell(shell, "", append(os.Environ(), "TERMBRIDGE_SPIKE=ok"))
		if err != nil {
			return err
		}
		defer s.close()
		if err := s.writeLine(shell, envCommand(shell)); err != nil {
			return err
		}
		if err := s.waitFor("TERM_BRIDGE_ENV_ok", timeout); err != nil {
			return err
		}
		if err := s.writeLine(shell, "exit"); err != nil {
			return err
		}
		return s.waitExit(timeout)
	})
}

func runShellResize(shell string, timeout time.Duration) result {
	return measure("resize "+shell, func() error {
		s, err := startShell(shell, "", nil)
		if err != nil {
			return err
		}
		defer s.close()
		if err := s.pty.Resize(120, 40); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
		if err := s.writeLine(shell, sizeCommand(shell)); err != nil {
			return err
		}
		if err := s.waitFor("TERM_BRIDGE_SIZE_120x40", timeout); err != nil {
			return err
		}
		if err := s.writeLine(shell, "exit"); err != nil {
			return err
		}
		return s.waitExit(timeout)
	})
}

func runCleanup(shell string, timeout time.Duration) result {
	return measure("cleanup "+shell, func() error {
		s, err := startShell(shell, "", nil)
		if err != nil {
			return err
		}
		if err := s.writeLine(shell, echoCommand(shell, "TERM_BRIDGE_CLEANUP_READY")); err != nil {
			_ = s.close()
			return err
		}
		if err := s.waitFor("TERM_BRIDGE_CLEANUP_READY", timeout); err != nil {
			_ = s.close()
			return err
		}
		if err := s.close(); err != nil {
			return err
		}
		return s.waitClosedExit(timeout)
	})
}

func runInterrupt(shell string, timeout time.Duration) result {
	return measure("interrupt "+shell, func() error {
		s, err := startShell(shell, "", nil)
		if err != nil {
			return err
		}
		defer s.close()
		if err := s.writeLine(shell, interruptCommand(shell)); err != nil {
			return err
		}
		if err := s.waitFor("TERM_BRIDGE_INTERRUPT_STARTED", timeout); err != nil {
			return err
		}
		if _, err := s.pty.Write([]byte{3}); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
		if err := s.writeLine(shell, echoCommand(shell, "TERM_BRIDGE_INTERRUPT_AFTER")); err != nil {
			return err
		}
		if err := s.waitFor("TERM_BRIDGE_INTERRUPT_AFTER", timeout); err != nil {
			return err
		}
		if err := s.writeLine(shell, "exit"); err != nil {
			return err
		}
		return s.waitExit(timeout)
	})
}

func runConcurrent(timeout time.Duration) result {
	return measure("concurrent sessions", func() error {
		shells := []string{"cmd", "powershell", "pwsh", "cmd", "powershell", "pwsh"}
		errs := make(chan error, len(shells))
		for i, shell := range shells {
			go func(index int, sh string) {
				marker := fmt.Sprintf("TERM_BRIDGE_CONCURRENT_%d", index)
				s, err := startShell(sh, "", nil)
				if err != nil {
					errs <- fmt.Errorf("%s start: %w", sh, err)
					return
				}
				defer s.close()
				if err := s.writeLine(sh, echoCommand(sh, marker)); err != nil {
					errs <- fmt.Errorf("%s write: %w", sh, err)
					return
				}
				if err := s.waitFor(marker, timeout); err != nil {
					errs <- fmt.Errorf("%s wait marker: %w", sh, err)
					return
				}
				if err := s.writeLine(sh, "exit"); err != nil {
					errs <- fmt.Errorf("%s exit write: %w", sh, err)
					return
				}
				errs <- s.waitExit(timeout)
			}(i, shell)
		}
		var failures []string
		for range shells {
			if err := <-errs; err != nil {
				failures = append(failures, err.Error())
			}
		}
		if len(failures) > 0 {
			return errors.New(strings.Join(failures, "; "))
		}
		return nil
	})
}

func runMultiAttach(shell string, timeout time.Duration) result {
	return measure("multi-attach "+shell, func() error {
		s, err := startShell(shell, "", nil)
		if err != nil {
			return err
		}
		defer s.close()
		attachA := s.attach()
		attachB := s.attach()
		defer s.detach(attachA)
		defer s.detach(attachB)
		marker := "TERM_BRIDGE_MULTI_ATTACH_" + strings.ToUpper(shell)
		if err := s.writeLine(shell, echoCommand(shell, marker)); err != nil {
			return err
		}
		if err := waitChannelFor(attachA, marker, timeout); err != nil {
			return fmt.Errorf("attach A: %w", err)
		}
		if err := waitChannelFor(attachB, marker, timeout); err != nil {
			return fmt.Errorf("attach B: %w", err)
		}
		if err := s.writeLine(shell, "exit"); err != nil {
			return err
		}
		return s.waitExit(timeout)
	})
}

func runOwnerCrash(timeout time.Duration) result {
	return measure("owner crash cleanup", func() error {
		marker := uniqueMarker("OWNER_CRASH")
		cmd := exec.Command(os.Args[0], "--internal-owner-crash-child", "--internal-marker", marker, "--timeout", timeout.String())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err == nil {
			return errors.New("owner crash child exited successfully; expected abrupt exit")
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 77 {
			return fmt.Errorf("unexpected owner crash child exit: %w", err)
		}
		time.Sleep(1200 * time.Millisecond)
		pids, err := processesWithMarker(marker)
		if err != nil {
			return err
		}
		if len(pids) > 0 {
			_ = killProcesses(pids)
			return fmt.Errorf("owner crash left descendant processes with marker %s: %v", marker, pids)
		}
		return nil
	})
}

func runOwnerCrashChild(marker string, timeout time.Duration) {
	if marker == "" {
		marker = uniqueMarker("OWNER_CRASH_CHILD")
	}
	s, err := startShell("cmd", "", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(76)
	}
	if err := s.writeLine("cmd", descendantCommand(marker)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(76)
	}
	if err := s.waitFor("TERM_BRIDGE_DESCENDANT_READY", timeout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(76)
	}
	os.Exit(77)
}

func runDescendantCleanup(shell string, timeout time.Duration) result {
	return measure("descendant cleanup "+shell, func() error {
		marker := uniqueMarker("DESCENDANT")
		s, err := startShell(shell, "", nil)
		if err != nil {
			return err
		}
		if err := s.writeLine(shell, descendantCommand(marker)); err != nil {
			_ = s.close()
			return err
		}
		if err := s.waitFor("TERM_BRIDGE_DESCENDANT_READY", timeout); err != nil {
			_ = s.close()
			return err
		}
		pids, err := processesWithMarker(marker)
		if err != nil {
			_ = s.close()
			return err
		}
		if len(pids) == 0 {
			_ = s.close()
			return fmt.Errorf("descendant marker process was not observable before close: %s", marker)
		}
		if err := s.close(); err != nil {
			_ = killProcesses(pids)
			return err
		}
		if err := s.waitClosedExit(timeout); err != nil {
			_ = killProcesses(pids)
			return err
		}
		time.Sleep(800 * time.Millisecond)
		remaining, err := processesWithMarker(marker)
		if err != nil {
			return err
		}
		if len(remaining) > 0 {
			_ = killProcesses(remaining)
			return fmt.Errorf("descendant processes remained after PTY close: %v", remaining)
		}
		return nil
	})
}

func runAgentVersion(command string, args []string, marker string, timeout time.Duration) result {
	return measure("agent "+command+" version", func() error {
		s, err := startCommand(resolveCommand(command), args, "", nil)
		if err != nil {
			return err
		}
		defer s.close()
		if err := s.waitFor(marker, timeout); err != nil {
			return err
		}
		return s.waitExit(timeout)
	})
}

func runTestProgram(name string, args []string, timeout time.Duration) result {
	return measure(name, func() error {
		program := filepath.Join(".tmp", "pty-testprogram.exe")
		if _, err := os.Stat(program); err != nil {
			return fmt.Errorf("missing %s; build it with go build -o .tmp/pty-testprogram.exe ./cmd/pty-testprogram: %w", program, err)
		}
		s, err := startCommand(program, args, "", nil)
		if err != nil {
			return err
		}
		defer s.close()
		marker := "TERM_BRIDGE_UNICODE_END"
		if name == "long-output" {
			marker = "TERM_BRIDGE_LONG_OUTPUT_END"
		}
		if err := s.waitFor(marker, timeout); err != nil {
			return err
		}
		return s.waitExit(timeout)
	})
}

func startShell(shell string, dir string, env []string) (*session, error) {
	switch shell {
	case "cmd":
		return startCommand(resolveCommand("cmd.exe"), nil, dir, env)
	case "powershell":
		return startCommand(resolveCommand("powershell.exe"), []string{"-NoLogo", "-NoProfile"}, dir, env)
	case "pwsh":
		return startCommand(resolveCommand("pwsh.exe"), []string{"-NoLogo", "-NoProfile"}, dir, env)
	default:
		return nil, fmt.Errorf("unsupported shell %q", shell)
	}
}

func resolveCommand(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return name
	}
	return path
}

func startCommand(command string, args []string, dir string, env []string) (*session, error) {
	pt, err := pty.New()
	if err != nil {
		return nil, err
	}
	cmd := pt.Command(command, args...)
	cmd.Dir = dir
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		_ = pt.Close()
		return nil, err
	}
	s := &session{pty: pt, cmd: cmd, done: make(chan error, 1), closed: make(chan struct{}), subs: make(map[chan []byte]struct{})}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pt.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				s.mu.Lock()
				_, _ = s.buf.Write(chunk)
				for sub := range s.subs {
					select {
					case sub <- chunk:
					default:
					}
				}
				s.mu.Unlock()
			}
			if err != nil {
				if !errors.Is(err, io.EOF) && !isClosed(s.closed) {
					s.mu.Lock()
					_, _ = s.buf.WriteString("\nTERM_BRIDGE_READ_ERROR: " + err.Error() + "\n")
					s.mu.Unlock()
				}
				return
			}
		}
	}()
	go func() { s.done <- cmd.Wait() }()
	return s, nil
}

func (s *session) attach() chan []byte {
	ch := make(chan []byte, 128)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

func (s *session) detach(ch chan []byte) {
	s.mu.Lock()
	delete(s.subs, ch)
	close(ch)
	s.mu.Unlock()
}

func (s *session) writeLine(shell string, command string) error {
	_, err := s.pty.Write([]byte(command + "\r\n"))
	return err
}

func (s *session) waitFor(substr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		current := s.buf.String()
		s.mu.Unlock()
		if strings.Contains(current, substr) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.mu.Lock()
	current := s.buf.String()
	s.mu.Unlock()
	if len(current) > 2000 {
		current = current[len(current)-2000:]
	}
	return fmt.Errorf("timed out waiting for %q; recent output:\n%s", substr, current)
}

func waitChannelFor(ch <-chan []byte, substr string, timeout time.Duration) error {
	var buf bytes.Buffer
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				return errors.New("attach channel closed")
			}
			_, _ = buf.Write(chunk)
			if strings.Contains(buf.String(), substr) {
				return nil
			}
		case <-timer.C:
			current := buf.String()
			if len(current) > 2000 {
				current = current[len(current)-2000:]
			}
			return fmt.Errorf("timed out waiting for %q; recent attach output:\n%s", substr, current)
		}
	}
}

func (s *session) waitExit(timeout time.Duration) error {
	select {
	case err := <-s.done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for process exit")
	}
}

func (s *session) waitClosedExit(timeout time.Duration) error {
	if err := s.waitExit(timeout); err != nil && !strings.Contains(err.Error(), "0xc000013a") {
		return err
	}
	return nil
}

func (s *session) close() error {
	select {
	case <-s.closed:
	default:
		close(s.closed)
	}
	return s.pty.Close()
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func echoCommand(shell string, value string) string {
	if shell == "cmd" {
		return "echo " + value
	}
	return "Write-Output " + value
}

func cwdCommand(shell string) string {
	if shell == "cmd" {
		return "cd"
	}
	return "(Get-Location).Path"
}

func envCommand(shell string) string {
	if shell == "cmd" {
		return "echo TERM_BRIDGE_ENV_%TERMBRIDGE_SPIKE%"
	}
	return "Write-Output (\"TERM_BRIDGE_ENV_$env:TERMBRIDGE_SPIKE\")"
}

func sizeCommand(shell string) string {
	if shell == "cmd" {
		return `powershell.exe -NoLogo -NoProfile -Command "$s=$Host.UI.RawUI.WindowSize; Write-Output ('TERM_BRIDGE_SIZE_' + $s.Width + 'x' + $s.Height)"`
	}
	return `$s=$Host.UI.RawUI.WindowSize; Write-Output ("TERM_BRIDGE_SIZE_$($s.Width)x$($s.Height)")`
}

func interruptCommand(shell string) string {
	if shell == "cmd" {
		return "echo TERM_BRIDGE_INTERRUPT_STARTED && ping -n 30 127.0.0.1"
	}
	return "Write-Output TERM_BRIDGE_INTERRUPT_STARTED; Start-Sleep -Seconds 30; Write-Output TERM_BRIDGE_INTERRUPT_NOT_STOPPED"
}

func descendantCommand(marker string) string {
	quoted := strings.ReplaceAll(marker, "'", "''")
	return `powershell.exe -NoLogo -NoProfile -Command "$env:TERMBRIDGE_CHILD_MARKER='` + quoted + `'; Write-Output 'TERM_BRIDGE_DESCENDANT_READY'; Start-Sleep -Seconds 60"`
}

func uniqueMarker(prefix string) string {
	return "TERMBRIDGE_" + prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func processesWithMarker(marker string) ([]int, error) {
	script := `$marker=$env:TERMBRIDGE_QUERY_MARKER; Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -and $_.CommandLine.Contains($marker) } | ForEach-Object { $_.ProcessId }`
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-Command", script)
	cmd.Env = append(os.Environ(), "TERMBRIDGE_QUERY_MARKER="+marker)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, field := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(strings.TrimSpace(field))
		if err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func killProcesses(pids []int) error {
	if len(pids) == 0 {
		return nil
	}
	args := []string{"-NoLogo", "-NoProfile", "-Command", "Stop-Process -Force -Id " + joinInts(pids)}
	return exec.Command("powershell.exe", args...).Run()
}

func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func measure(name string, fn func() error) result {
	started := time.Now()
	err := fn()
	item := result{Name: name, Passed: err == nil, Duration: time.Since(started)}
	if err != nil {
		item.Error = err.Error()
	}
	return item
}
