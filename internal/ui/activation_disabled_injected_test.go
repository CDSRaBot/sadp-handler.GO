package ui

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

func TestTUIShowsDisabledMessageForInjectedSADPService(t *testing.T) {
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{
		Activation: WorkflowActivationService{},
		Config:     RuntimeUIConfig{Protocol: "sadp_first"},
	})
	model.console.SetValue("activate")
	command := model.submitConsole()
	if command == nil || model.pending != nil {
		t.Fatal("disabled activation must not require target selection or confirmation")
	}
	updated, _ := model.Update(command())
	if view := updated.(TUIModel).View(); !strings.Contains(view, "SADP activation is disabled") {
		t.Fatalf("disabled message not visible: %q", view)
	}
}

func TestTUIISAPIOnlyReportsUnavailableWorkflow(t *testing.T) {
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{
		Config: RuntimeUIConfig{Protocol: "isapi_only"},
	})
	model.console.SetValue("activate")
	command := model.submitConsole()
	if command == nil {
		t.Fatal("unavailable ISAPI workflow did not produce a result")
	}
	updated, _ := model.Update(command())
	view := updated.(TUIModel).View()
	if !strings.Contains(view, "activate workflow is not implemented") || strings.Contains(view, "SADP activation is disabled") {
		t.Fatalf("unexpected isapi-only message: %q", view)
	}
}

func TestREPLInjectedSADPServiceAndISAPIOnly(t *testing.T) {
	for _, tt := range []struct {
		name         string
		dependencies Dependencies
		want         string
	}{
		{"disabled service", Dependencies{Activation: WorkflowActivationService{}, Config: RuntimeUIConfig{Protocol: "sadp_first"}}, "SADP activation is disabled"},
		{"unavailable ISAPI", Dependencies{Config: RuntimeUIConfig{Protocol: "isapi_only"}}, "activate workflow is not implemented"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := RunCLIWithTerminalState(context.Background(), []string{"--ui=no", "--protocol=" + tt.dependencies.Config.Protocol},
				strings.NewReader("activate\nquit\n"), &stdout, &stderr, true, false, tt.dependencies)
			if exit != ExitSuccess || !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
			}
		})
	}
}

func TestTUIAvailableActivationStillRequiresConfirmation(t *testing.T) {
	state := NewTUIState()
	state.SetDevices([]parser.Device{
		{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}},
		{MAC: net.HardwareAddr{6, 7, 8, 9, 10, 11}},
	})
	state.ToggleFocusedSelection()
	state.MoveFocus(1)
	state.ToggleFocusedSelection()
	called := false
	model := NewTUIModel(context.Background(), state, Dependencies{
		Activation: activationServiceFunc(func(context.Context, ActivateCommand) ([]DeviceOutcome, error) {
			called = true
			return nil, nil
		}),
	})
	model.console.SetValue("activate")
	if command := model.submitConsole(); command != nil || model.pending == nil || called {
		t.Fatal("available activation must require bulk confirmation before dispatch")
	}
}
