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

func TestWriteTextResultRendersDiscoveryAndMutationOutcomes(t *testing.T) {
	var discoveryOutput, discoveryError bytes.Buffer
	exitCode := ui.RunCLIWithTerminalState(context.Background(), []string{"--ui=no", "--once", "discover", "--interface", "1", "--timeout", "1s"},
		strings.NewReader(""), &discoveryOutput, &discoveryError, false, false, ui.Dependencies{Discovery: runtimeDiscovery{}})
	if exitCode != ui.ExitSuccess {
		t.Fatalf("discovery exit code: %d; output=%q; error=%q", exitCode, discoveryOutput.String(), discoveryError.String())
	}
	if !strings.Contains(discoveryOutput.String(), "00:11:22:33:44:55 192.0.2.10 DS-1 192.0.2.1") {
		t.Fatalf("discovery output = %q", discoveryOutput.String())
	}

	var mutationOutput bytes.Buffer
	exitCode = ui.RunCLIWithTerminalState(context.Background(), []string{"--ui=no", "--once", "activate", "--device", "00:11:22:33:44:55", "--password-stdin"},
		strings.NewReader("secret\n"), &mutationOutput, &bytes.Buffer{}, false, false, ui.Dependencies{Activation: runtimeActivation{}})
	if exitCode != ui.ExitSuccess {
		t.Fatalf("mutation exit code: %d", exitCode)
	}
	if !strings.Contains(mutationOutput.String(), "00:11:22:33:44:55 activated") {
		t.Fatalf("mutation output = %q", mutationOutput.String())
	}
}

type runtimeDiscovery struct{}

func (runtimeDiscovery) Discover(context.Context, *int, ui.DiscoveryPolicy) ([]ui.DiscoveredDevice, error) {
	return []ui.DiscoveredDevice{{MAC: "00:11:22:33:44:55", IPv4: "192.0.2.10", Model: "DS-1", Sender: "192.0.2.1"}}, nil
}

type runtimeActivation struct{}

func (runtimeActivation) Activate(context.Context, ui.ActivateCommand) ([]ui.DeviceOutcome, error) {
	return []ui.DeviceOutcome{{MAC: "00:11:22:33:44:55", Status: "activated"}}, nil
}
