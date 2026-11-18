package ui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestISAPIStatusInteractiveFailureIsSafe(t *testing.T) {
	service := isapiStatusServiceFunc(func(context.Context, ISAPIStatusCommand) (ISAPIStatusResult, error) {
		return ISAPIStatusResult{}, errors.New("synthetic-secret")
	})
	line := "isapi-status --address 192.0.2.1"
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{ISAPIStatus: service})
	model.console.SetValue(line)
	command := model.submitConsole()
	if command == nil {
		t.Fatal("status command not dispatched")
	}
	updated, _ := model.Update(command())
	view := updated.(TUIModel).View()
	if !strings.Contains(view, "ISAPI status failed") || strings.Contains(view, "synthetic-secret") {
		t.Fatalf("unexpected TUI failure: %q", view)
	}
	var stdout, stderr bytes.Buffer
	exit := RunCLIWithTerminalState(context.Background(), []string{"--ui=no"},
		strings.NewReader(line+"\nquit\n"), &stdout, &stderr, true, false,
		Dependencies{ISAPIStatus: service})
	if exit != ExitSuccess || !strings.Contains(stdout.String(), "ISAPI status failed") ||
		strings.Contains(stdout.String()+stderr.String(), "synthetic-secret") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestISAPIStatusCanceledBeforeService(t *testing.T) {
	called := false
	service := isapiStatusServiceFunc(func(context.Context, ISAPIStatusCommand) (ISAPIStatusResult, error) {
		called = true
		return ISAPIStatusResult{}, nil
	})
	command, err := ParseConsoleCommand("isapi-status --address 192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := DispatchCommand(ctx, command, Dependencies{ISAPIStatus: service})
	if !errors.Is(err, context.Canceled) || result.Error == nil || result.Error.Code != FailureCanceled || called {
		t.Fatalf("result=%+v err=%v called=%v", result, err, called)
	}
}
