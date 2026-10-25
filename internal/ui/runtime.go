package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"
)

const (
	FormatText = "text"
	FormatJSON = "json"

	ExitSuccess      = 0
	ExitOperational  = 1
	ExitInvalidInput = 2
	ExitInterrupted  = 130
)

func ParseRuntimeArgs(args []string) (RuntimeUIConfig, []string, error) {
	return ParseRuntimeArgsWithDefaults(args, RuntimeUIConfig{Mode: "yes", Format: FormatText, Language: "en"})
}

func ParseRuntimeArgsWithDefaults(args []string, defaults RuntimeUIConfig) (RuntimeUIConfig, []string, error) {
	config := defaults
	var commandArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--once":
			if config.Once {
				return RuntimeUIConfig{}, nil, errors.New("--once cannot be repeated")
			}
			config.Once = true
		case arg == "--ui" || arg == "--format" || arg == "--lang":
			if i+1 >= len(args) {
				return RuntimeUIConfig{}, nil, fmt.Errorf("%s requires a value", arg)
			}
			value := args[i+1]
			i++
			if err := applyRuntimeOption(&config, arg, value); err != nil {
				return RuntimeUIConfig{}, nil, err
			}
		case strings.HasPrefix(arg, "--ui="):
			if err := applyRuntimeOption(&config, "--ui", strings.TrimPrefix(arg, "--ui=")); err != nil {
				return RuntimeUIConfig{}, nil, err
			}
		case strings.HasPrefix(arg, "--format="):
			if err := applyRuntimeOption(&config, "--format", strings.TrimPrefix(arg, "--format=")); err != nil {
				return RuntimeUIConfig{}, nil, err
			}
		case strings.HasPrefix(arg, "--lang="):
			if err := applyRuntimeOption(&config, "--lang", strings.TrimPrefix(arg, "--lang=")); err != nil {
				return RuntimeUIConfig{}, nil, err
			}
		default:
			commandArgs = append(commandArgs, args[i:]...)
			i = len(args)
		}
	}
	if config.Mode == "yes" && config.Once {
		return RuntimeUIConfig{}, nil, errors.New("--once requires --ui=no")
	}
	if config.Mode == "yes" && len(commandArgs) > 0 {
		return RuntimeUIConfig{}, nil, errors.New("commands require --ui=no --once")
	}
	if config.Mode == "no" && !config.Once && len(commandArgs) > 0 {
		return RuntimeUIConfig{}, nil, errors.New("positional commands require --once")
	}
	if config.Once && len(commandArgs) == 0 {
		return RuntimeUIConfig{}, nil, errors.New("--once requires one command")
	}
	if config.Once {
		if _, err := ParseCLICommand(commandArgs); err != nil {
			return RuntimeUIConfig{}, nil, err
		}
	}
	return config, commandArgs, nil
}

func applyRuntimeOption(config *RuntimeUIConfig, option, value string) error {
	if value == "" {
		return fmt.Errorf("%s value is empty", option)
	}
	switch option {
	case "--ui":
		if value != "yes" && value != "no" {
			return errors.New("--ui must be yes or no")
		}
		config.Mode = value
	case "--format":
		if value != FormatText && value != FormatJSON {
			return errors.New("--format must be text or json")
		}
		config.Format = value
	case "--lang":
		config.Language = value
	default:
		return errors.New("unknown runtime option")
	}
	return nil
}

func RunCLI(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, dependencies Dependencies) int {
	return RunCLIWithDefaults(ctx, args, stdin, stdout, stderr, RuntimeUIConfig{Mode: "yes", Format: FormatText, Language: "en"}, dependencies)
}

func RunCLIWithTerminalState(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, stdinIsTerminal, stdoutIsTerminal bool, dependencies Dependencies) int {
	return runCLI(ctx, args, stdin, stdout, stderr, dependencies, stdinIsTerminal, stdoutIsTerminal, RuntimeUIConfig{Mode: "yes", Format: FormatText, Language: "en"})
}

func RunCLIWithDefaults(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, defaults RuntimeUIConfig, dependencies Dependencies) int {
	return runCLI(ctx, args, stdin, stdout, stderr, dependencies, streamIsTerminal(stdin), streamIsTerminal(stdout), defaults)
}

func runCLI(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, dependencies Dependencies, stdinIsTerminal, stdoutIsTerminal bool, defaults RuntimeUIConfig) int {
	config, commandArgs, err := ParseRuntimeArgsWithDefaults(args, defaults)
	if err != nil {
		writeDiagnostic(stderr, err.Error())
		return ExitInvalidInput
	}
	dependencies.Config = config
	if stdin == nil || stdout == nil || stderr == nil {
		return ExitInvalidInput
	}
	mode, err := SelectUIMode(config.Mode, config.Once, stdinIsTerminal, stdoutIsTerminal)
	if err != nil {
		writeDiagnostic(stderr, err.Error())
		return ExitInvalidInput
	}
	if mode == ModeTUI {
		if err := RunTUI(ctx, dependencies); err != nil {
			writeDiagnostic(stderr, "TUI failed")
			return ExitOperational
		}
		return ExitSuccess
	}
	if config.Once {
		return runOneShot(ctx, commandArgs, stdout, stderr, dependencies)
	}
	return runREPL(ctx, stdin, stdout, stderr, dependencies)
}

func streamIsTerminal(stream any) bool {
	file, ok := stream.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	fd := file.Fd()
	return term.IsTerminal(int(fd))
}

func runOneShot(ctx context.Context, args []string, stdout, stderr io.Writer, dependencies Dependencies) int {
	command, err := ParseCLICommand(args)
	if err != nil {
		writeDiagnostic(stderr, err.Error())
		return ExitInvalidInput
	}
	result, err := DispatchCommand(ctx, command, dependencies)
	if writeResult(stdout, dependencies.Config.Format, result) != nil {
		writeDiagnostic(stderr, "write command result failed")
		return ExitOperational
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ExitInterrupted
	}
	if err != nil || !result.OK {
		return ExitOperational
	}
	return ExitSuccess
}

func runREPL(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, dependencies Dependencies) int {
	scanner := bufio.NewScanner(stdin)
	for {
		if _, err := fmt.Fprint(stderr, "> "); err != nil {
			return ExitOperational
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				writeDiagnostic(stderr, "read input failed")
				return ExitOperational
			}
			return ExitSuccess
		}
		command, err := ParseConsoleCommand(scanner.Text())
		if err != nil {
			result := CommandResult{OK: false, Error: &CommandFailure{Code: FailureInvalidArgument, Message: "invalid command"}}
			if writeResult(stdout, dependencies.Config.Format, result) != nil {
				return ExitOperational
			}
			continue
		}
		result, dispatchErr := DispatchCommand(ctx, command, dependencies)
		if err := writeResult(stdout, dependencies.Config.Format, result); err != nil {
			writeDiagnostic(stderr, "write command result failed")
			return ExitOperational
		}
		if dispatchErr != nil && result.Error == nil {
			writeDiagnostic(stderr, "command failed")
		}
		if result.Exit {
			return ExitSuccess
		}
	}
}

func writeResult(writer io.Writer, format string, result CommandResult) error {
	if format == FormatJSON {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(writer, string(data))
		return err
	}
	return writeTextResult(writer, result)
}

func writeTextResult(writer io.Writer, result CommandResult) error {
	if result.Error != nil {
		_, err := fmt.Fprintf(writer, "%s: %s\n", result.Error.Code, result.Error.Message)
		return err
	}
	if result.Message != "" {
		_, err := fmt.Fprintln(writer, result.Message)
		return err
	}
	if result.Version != "" {
		_, err := fmt.Fprintln(writer, result.Version)
		return err
	}
	if len(result.Locales) > 0 {
		_, err := fmt.Fprintln(writer, strings.Join(result.Locales, "\n"))
		return err
	}
	return nil
}

func writeDiagnostic(writer io.Writer, message string) {
	if writer != nil {
		_, _ = fmt.Fprintln(writer, message)
	}
}
