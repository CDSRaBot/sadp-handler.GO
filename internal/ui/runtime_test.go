package ui_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

func TestParseRuntimeArgsSeparatesGlobalOptionsAndOneShotCommand(t *testing.T) {
	config, command, err := ui.ParseRuntimeArgs([]string{
		"--ui=no", "--format=json", "--lang", "ru", "--once", "version",
	})
	if err != nil {
		t.Fatalf("ParseRuntimeArgs returned error: %v", err)
	}
	if config.Mode != "no" || config.Format != ui.FormatJSON || config.Language != "ru" || !config.Once {
		t.Fatalf("config = %#v", config)
	}
	if len(command) != 1 || command[0] != "version" {
		t.Fatalf("command = %#v", command)
	}
}

func TestParseRuntimeArgsRejectsInvalidInvocation(t *testing.T) {
	for _, args := range [][]string{
		{"--once"},
		{"--once", "version"},
		{"--ui=no", "--format=xml", "--once", "version"},
		{"--ui=no", "version"},
		{"--ui=no", "--once", "version", "help"},
	} {
		if _, _, err := ui.ParseRuntimeArgs(args); err == nil {
			t.Fatalf("ParseRuntimeArgs(%#v) succeeded; want error", args)
		}
	}
}

func TestRunCLIJSONKeepsResultsOnStdoutAndDiagnosticsOnStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := ui.RunCLI(context.Background(), []string{"--ui=no", "--format=json", "--once", "version"},
		strings.NewReader(""), &stdout, &stderr, ui.Dependencies{Version: "1.2.3"})
	if exitCode != ui.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, ui.ExitSuccess, stderr.String())
	}
	var result ui.CommandResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not one JSON result: %v; output=%q", err, stdout.String())
	}
	if result.Version != "1.2.3" || stderr.Len() != 0 {
		t.Fatalf("result=%#v stderr=%q", result, stderr.String())
	}
}

func TestRunCLIREPLContinuesAfterParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := ui.RunCLIWithTerminalState(context.Background(), []string{"--ui=no", "--format=json"},
		strings.NewReader("bad-command\nquit\n"), &stdout, &stderr, true, false, ui.Dependencies{})
	if exitCode != ui.ExitSuccess {
		t.Fatalf("exit code = %d; stderr=%q", exitCode, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout lines = %#v; stderr=%q", lines, stderr.String())
	}
	for _, line := range lines {
		var result ui.CommandResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			t.Fatalf("invalid JSON result %q: %v", line, err)
		}
	}
}
