package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"termbridge-go/internal/app"
	apperrors "termbridge-go/internal/errors"
	"termbridge-go/internal/version"
)

type CommandKind string

const (
	CommandExec      CommandKind = "exec"
	CommandWorkspace CommandKind = "workspace"
	CommandSession   CommandKind = "session"
	CommandWeb       CommandKind = "web"
)

type ExecOptions struct {
	Command  []string
	ShowHelp bool
}

type WebOptions struct {
	Host     string
	Port     int
	Open     bool
	Dev      bool
	ShowHelp bool
}

type Options struct {
	Cwd         string
	Kind        CommandKind
	Exec        ExecOptions
	Web         WebOptions
	ShowHelp    bool
	ShowVersion bool
}

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	options, err := Parse(args, stderr)
	if err != nil {
		printError(stderr, err)
		PrintUsage(stderr)
		return apperrors.ExitCode(err)
	}

	if options.ShowHelp {
		PrintUsage(stdout)
		return apperrors.ExitSuccess
	}

	if options.ShowVersion {
		fmt.Fprintln(stdout, version.String())
		return apperrors.ExitSuccess
	}

	if options.Exec.ShowHelp {
		PrintExecUsage(stdout)
		return apperrors.ExitSuccess
	}

	if options.Web.ShowHelp {
		PrintWebUsage(stdout)
		return apperrors.ExitSuccess
	}

	result, err := app.Run(context.Background(), app.Options{
		Cwd: options.Cwd,
		Command: app.Command{
			Kind: app.CommandKind(options.Kind),
			Exec: app.ExecCommand{Command: options.Exec.Command},
			Web:  app.WebCommand{Host: options.Web.Host, Port: options.Web.Port, Open: options.Web.Open, Dev: options.Web.Dev},
		},
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		printError(stderr, err)
		if apperrors.KindOf(err) == apperrors.KindRuntime {
			fmt.Fprintf(stderr, "cwd: %s\n", result.Cwd)
			fmt.Fprintf(stderr, "command: %s\n", strings.Join(result.Command, " "))
		}
		return apperrors.ExitCode(err)
	}

	if result.ExitCode != 0 {
		return result.ExitCode
	}
	return apperrors.ExitSuccess
}

func Parse(args []string, output io.Writer) (Options, error) {
	options := Options{}
	flags := flag.NewFlagSet("termbridge", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&options.ShowHelp, "help", false, "show help")
	flags.BoolVar(&options.ShowVersion, "version", false, "show version")
	flags.StringVar(&options.Cwd, "cwd", "", "working directory for TermBridge")

	if len(args) > 0 && args[0] == "--" {
		return Options{}, apperrors.Usage("old command form is no longer supported; use: termbridge exec -- <command>")
	}

	commandIndex := len(args)
	for i, arg := range args {
		if isCommand(arg) {
			commandIndex = i
			break
		}
	}
	if commandIndex == len(args) {
		if err := flags.Parse(args); err != nil {
			return Options{}, apperrors.Usage(err.Error())
		}
		if options.ShowHelp || options.ShowVersion {
			return options, nil
		}
		if len(flags.Args()) > 0 {
			return Options{}, apperrors.Usage("unknown command: " + strings.Join(flags.Args(), " "))
		}
		return Options{}, apperrors.Usage("missing command")
	}

	if err := flags.Parse(args[:commandIndex]); err != nil {
		return Options{}, apperrors.Usage(err.Error())
	}
	if len(flags.Args()) > 0 {
		return Options{}, apperrors.Usage("unexpected arguments before command: " + strings.Join(flags.Args(), " "))
	}

	command := args[commandIndex]
	rest := args[commandIndex+1:]
	switch command {
	case "exec":
		options.Kind = CommandExec
		return parseExec(options, rest, output)
	case "workspace":
		options.Kind = CommandWorkspace
		if len(rest) > 0 {
			return Options{}, apperrors.Usage("workspace does not accept arguments")
		}
		return options, nil
	case "session":
		options.Kind = CommandSession
		if len(rest) > 0 {
			return Options{}, apperrors.Usage("session does not accept arguments")
		}
		return options, nil
	case "web":
		options.Kind = CommandWeb
		return parseWeb(options, rest, output)
	default:
		return Options{}, apperrors.Usage("unknown command: " + command)
	}
}

func parseWeb(options Options, args []string, output io.Writer) (Options, error) {
	if len(args) == 1 && args[0] == "--help" {
		options.Web.ShowHelp = true
		return options, nil
	}
	flags := flag.NewFlagSet("termbridge web", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&options.Web.Host, "host", "127.0.0.1", "web server host")
	flags.IntVar(&options.Web.Port, "port", 0, "web server port")
	flags.BoolVar(&options.Web.Open, "open", false, "open browser after server starts")
	flags.BoolVar(&options.Web.Dev, "dev", false, "enable development server friendly behavior")
	flags.BoolVar(&options.Web.ShowHelp, "help", false, "show web help")
	if err := flags.Parse(args); err != nil {
		return Options{}, apperrors.Usage(err.Error())
	}
	if options.Web.ShowHelp {
		return options, nil
	}
	if len(flags.Args()) > 0 {
		return Options{}, apperrors.Usage("web does not accept positional arguments: " + strings.Join(flags.Args(), " "))
	}
	if options.Web.Port < 0 || options.Web.Port > 65535 {
		return Options{}, apperrors.Usage("web port must be between 0 and 65535")
	}
	return options, nil
}

func parseExec(options Options, args []string, output io.Writer) (Options, error) {
	if len(args) == 1 && args[0] == "--help" {
		options.Exec.ShowHelp = true
		return options, nil
	}
	flags := flag.NewFlagSet("termbridge exec", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&options.Exec.ShowHelp, "help", false, "show exec help")

	separator := len(args)
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if err := flags.Parse(args[:separator]); err != nil {
		return Options{}, apperrors.Usage(err.Error())
	}
	if options.Exec.ShowHelp {
		return options, nil
	}
	if len(flags.Args()) > 0 {
		return Options{}, apperrors.Usage("unexpected exec arguments before --: " + strings.Join(flags.Args(), " "))
	}
	if separator == len(args) {
		return Options{}, apperrors.Usage("missing -- before exec command")
	}
	options.Exec.Command = args[separator+1:]
	if len(options.Exec.Command) == 0 {
		return Options{}, apperrors.Usage("missing command after --")
	}
	return options, nil
}

func isCommand(arg string) bool {
	switch arg {
	case "exec", "workspace", "session", "web":
		return true
	default:
		return false
	}
}

func PrintUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  termbridge [options] <command> [command options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  exec       run a command through a PTY")
	fmt.Fprintln(w, "  workspace  list workspaces")
	fmt.Fprintln(w, "  session    list sessions")
	fmt.Fprintln(w, "  web        start local Web terminal server")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --cwd <dir>     working directory for TermBridge; defaults to current directory")
	fmt.Fprintln(w, "  --version       show version")
	fmt.Fprintln(w, "  --help          show help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Config files:")
	fmt.Fprintln(w, "  TermBridge reads .termbridge.default.yaml, then .termbridge.yaml from --cwd/current directory.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  termbridge exec -- claude")
	fmt.Fprintln(w, "  termbridge --cwd D:\\project exec -- codex")
	fmt.Fprintln(w, "  termbridge --cwd D:\\project exec -- pwsh")
	fmt.Fprintln(w, "  termbridge web --dev")
}

func PrintWebUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  termbridge [options] web [web options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --host <host>    web server host; defaults to 127.0.0.1")
	fmt.Fprintln(w, "  --port <port>    web server port; defaults to 0 (auto-select)")
	fmt.Fprintln(w, "  --open           open browser after server starts")
	fmt.Fprintln(w, "  --dev            enable Vite dev-server friendly behavior")
	fmt.Fprintln(w, "  --help           show web help")
}

func PrintExecUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  termbridge [options] exec [exec options] -- <command> [args...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --help          show exec help")
}

func printError(w io.Writer, err error) {
	fmt.Fprintf(w, "error: %s\n\n", apperrors.FormatUser(err))
}
