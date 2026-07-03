package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"termbridge-go/cmd/termbridge/app"
	apperrors "termbridge-go/internal/shared/errors"
	"termbridge-go/internal/shared/version"
)

type CommandKind string

const (
	CommandExec      CommandKind = "exec"
	CommandWorkspace CommandKind = "workspace"
	CommandSession   CommandKind = "session"
	CommandAgent     CommandKind = "agent"
	CommandCloud     CommandKind = "cloud"
	CommandMigrate   CommandKind = "migrate"
)

type ExecOptions struct {
	Command  []string
	ShowHelp bool
}

type RoleOptions struct {
	ShowHelp bool
}

type MigrateOptions struct {
	Role     string
	ShowHelp bool
}

type Options struct {
	Cwd         string
	Kind        CommandKind
	Exec        ExecOptions
	Role        RoleOptions
	Migrate     MigrateOptions
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

	if options.Role.ShowHelp {
		PrintRoleUsage(stdout, string(options.Kind))
		return apperrors.ExitSuccess
	}

	if options.Migrate.ShowHelp {
		PrintMigrateUsage(stdout)
		return apperrors.ExitSuccess
	}

	result, err := app.Run(context.Background(), app.Options{
		Cwd: options.Cwd,
		Command: app.Command{
			Kind:        app.CommandKind(options.Kind),
			Exec:        app.ExecCommand{Command: options.Exec.Command},
			MigrateRole: options.Migrate.Role,
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
	case "agent":
		options.Kind = CommandAgent
		return parseRole(options, "termbridge agent", rest, output)
	case "cloud":
		options.Kind = CommandCloud
		return parseRole(options, "termbridge cloud", rest, output)
	case "migrate":
		options.Kind = CommandMigrate
		return parseMigrate(options, rest, output)
	default:
		return Options{}, apperrors.Usage("unknown command: " + command)
	}
}

func parseRole(options Options, name string, args []string, output io.Writer) (Options, error) {
	if len(args) == 1 && args[0] == "--help" {
		options.Role.ShowHelp = true
		return options, nil
	}
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&options.Role.ShowHelp, "help", false, "show help")
	if err := flags.Parse(args); err != nil {
		return Options{}, apperrors.Usage(err.Error())
	}
	if options.Role.ShowHelp {
		return options, nil
	}
	if len(flags.Args()) > 0 {
		return Options{}, apperrors.Usage(name + " does not accept positional arguments: " + strings.Join(flags.Args(), " "))
	}
	return options, nil
}

func parseMigrate(options Options, args []string, output io.Writer) (Options, error) {
	if len(args) == 1 && args[0] == "--help" {
		options.Migrate.ShowHelp = true
		return options, nil
	}
	flags := flag.NewFlagSet("termbridge migrate", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&options.Migrate.ShowHelp, "help", false, "show migrate help")
	if err := flags.Parse(args); err != nil {
		return Options{}, apperrors.Usage(err.Error())
	}
	if options.Migrate.ShowHelp {
		return options, nil
	}
	if len(flags.Args()) != 1 {
		return Options{}, apperrors.Usage("migrate requires role: termbridge migrate agent|cloud")
	}
	role := flags.Args()[0]
	if role != "agent" && role != "cloud" {
		return Options{}, apperrors.Usage("unknown migrate role: " + role)
	}
	options.Migrate.Role = role
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
	case "exec", "workspace", "session", "agent", "cloud", "migrate":
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
	fmt.Fprintln(w, "  agent      start local agent server, runtime, and optional cloud connector")
	fmt.Fprintln(w, "  cloud      start cloud API, device binding, and tunnel ingress")
	fmt.Fprintln(w, "  exec       run a command through a PTY")
	fmt.Fprintln(w, "  workspace  list workspaces")
	fmt.Fprintln(w, "  session    list sessions")
	fmt.Fprintln(w, "  migrate    run role-specific database migrations")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --cwd <dir>     working directory for TermBridge; defaults to current directory")
	fmt.Fprintln(w, "  --version       show version")
	fmt.Fprintln(w, "  --help          show help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Config files:")
	fmt.Fprintln(w, "  TermBridge reads configs/config.yaml from --cwd/current directory.")
	fmt.Fprintln(w, "  Set TERMBRIDGE_ENV=<env> to also read configs/config.<env>.yaml and .env.<env>.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  termbridge agent")
	fmt.Fprintln(w, "  termbridge cloud")
	fmt.Fprintln(w, "  termbridge migrate agent")
	fmt.Fprintln(w, "  termbridge migrate cloud")
	fmt.Fprintln(w, "  termbridge exec -- claude")
	fmt.Fprintln(w, "  termbridge --cwd D:\\project exec -- codex")
	fmt.Fprintln(w, "  termbridge --cwd D:\\project exec -- pwsh")
}

func PrintExecUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  termbridge [options] exec [exec options] -- <command> [args...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --help          show exec help")
}

func PrintRoleUsage(w io.Writer, command string) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  termbridge [options] %s\n", command)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s starts the %s business entry.\n", command, command)
	fmt.Fprintln(w, "Listen settings and role-specific dependencies are read from config.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --help          show help")
}

func PrintMigrateUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  termbridge [options] migrate <agent|cloud>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --help          show migrate help")
}

func printError(w io.Writer, err error) {
	fmt.Fprintf(w, "error: %s\n\n", apperrors.FormatUser(err))
}
