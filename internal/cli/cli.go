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

type Options struct {
	Cwd         string
	Command     []string
	ShowHelp    bool
	ShowVersion bool
}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
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

	result, err := app.Run(context.Background(), app.Options{
		Cwd:     options.Cwd,
		Command: options.Command,
	})
	if err != nil {
		printError(stderr, err)
		if apperrors.KindOf(err) == apperrors.KindRuntime {
			fmt.Fprintf(stderr, "cwd: %s\n", result.Cwd)
			fmt.Fprintf(stderr, "command: %s\n", strings.Join(result.Command, " "))
		}
		return apperrors.ExitCode(err)
	}

	return apperrors.ExitSuccess
}

func Parse(args []string, output io.Writer) (Options, error) {
	options := Options{}
	flags := flag.NewFlagSet("termbridge", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&options.ShowHelp, "help", false, "show help")
	flags.BoolVar(&options.ShowVersion, "version", false, "show version")
	flags.StringVar(&options.Cwd, "cwd", "", "working directory for the command")

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

	if separator < len(args) {
		options.Command = args[separator+1:]
	}

	if separator == len(args) && !options.ShowHelp && !options.ShowVersion {
		return Options{}, apperrors.Usage("missing -- before command")
	}
	if separator < len(args) && len(options.Command) == 0 && !options.ShowHelp && !options.ShowVersion {
		return Options{}, apperrors.Usage("missing command after --")
	}
	if len(flags.Args()) > 0 {
		return Options{}, apperrors.Usage("unexpected arguments before --: " + strings.Join(flags.Args(), " "))
	}

	return options, nil
}

func PrintUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  termbridge [options] -- <command> [args...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --cwd <dir>     working directory for the command; defaults to current directory")
	fmt.Fprintln(w, "  --version       show version")
	fmt.Fprintln(w, "  --help          show help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Config files:")
	fmt.Fprintln(w, "  TermBridge reads .termbridge.yaml from --cwd/current directory, then from the user home directory.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  termbridge -- claude")
	fmt.Fprintln(w, "  termbridge --cwd D:\\project -- codex")
	fmt.Fprintln(w, "  termbridge --cwd D:\\project -- pwsh")
}

func printError(w io.Writer, err error) {
	fmt.Fprintf(w, "error: %s\n\n", apperrors.FormatUser(err))
}
