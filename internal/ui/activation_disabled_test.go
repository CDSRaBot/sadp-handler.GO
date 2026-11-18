package ui

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

func TestTUIShowsDisabledSADPActivationWithoutTargetOrConfirmation(t *testing.T) {
	for _, tt := range []struct {
		name       string
		line       string
		selectMany bool
	}{
		{name: "no devices", line: "activate"},
		{name: "explicit MAC", line: "activate 00:01:02:03:04:05"},
		{name: "selected devices", line: "activate", selectMany: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := NewTUIState()
			if tt.selectMany {
				state.SetDevices([]parser.Device{
					{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}},
					{MAC: net.HardwareAddr{6, 7, 8, 9, 10, 11}},
				})
				state.ToggleFocusedSelection()
				state.MoveFocus(1)
				state.ToggleFocusedSelection()
			}
			model := NewTUIModel(context.Background(), state, Dependencies{Config: RuntimeUIConfig{Protocol: "sadp_first"}})
			model.console.SetValue(tt.line)
			command := model.submitConsole()
			if command == nil || model.pending != nil {
				t.Fatal("disabled activation must not require target selection or confirmation")
			}
			updated, _ := model.Update(command())
			result := updated.(TUIModel)
			if !strings.Contains(result.View(), "SADP activation is disabled") {
				t.Fatalf("disabled message not visible: %q", result.View())
			}
		})
	}
}

func TestREPLShowsDisabledSADPActivation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := RunCLIWithTerminalState(context.Background(), []string{"--ui=no"},
		strings.NewReader("activate\nquit\n"), &stdout, &stderr, true, false,
		Dependencies{Config: RuntimeUIConfig{Protocol: "sadp_first"}})
	if exit != ExitSuccess || !strings.Contains(stdout.String(), "SADP activation is disabled") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestTUIPrefersPublicCommandFailureOverInternalCause(t *testing.T) {
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{})
	updated, _ := model.Update(tuiCommandResultMsg{
		result: CommandResult{Error: &CommandFailure{Message: "safe message"}},
		err:    errors.New("synthetic-secret"),
	})
	view := updated.(TUIModel).View()
	if !strings.Contains(view, "safe message") || strings.Contains(view, "synthetic-secret") {
		t.Fatalf("unexpected TUI failure output: %q", view)
	}
}

func TestDisabledActivationDoesNotBypassInjectedServiceValidation(t *testing.T) {
	called := false
	dependencies := Dependencies{Activation: activationServiceFunc(func(context.Context, ActivateCommand) ([]DeviceOutcome, error) {
		called = true
		return nil, nil
	})}
	result, err := DispatchCommand(context.Background(), ActivateCommand{}, dependencies)
	if err == nil || result.Error == nil || result.Error.Code != FailureInvalidArgument || called {
		t.Fatalf("invalid activation accepted: %+v, %v, called=%v", result, err, called)
	}
}

type activationServiceFunc func(context.Context, ActivateCommand) ([]DeviceOutcome, error)

func (f activationServiceFunc) Activate(ctx context.Context, command ActivateCommand) ([]DeviceOutcome, error) {
	return f(ctx, command)
}
