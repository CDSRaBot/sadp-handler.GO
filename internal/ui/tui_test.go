package ui_test

import (
	"context"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTUIModelRendersAndHandlesCancellationWithoutShellExecution(t *testing.T) {
	model := ui.NewTUIModel(context.Background(), ui.NewTUIState(), ui.Dependencies{})
	view := model.View()
	if !strings.Contains(view, "SADP Handler") || !strings.Contains(view, "Devices") {
		t.Fatalf("unexpected initial TUI view: %q", view)
	}
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if command == nil {
		t.Fatal("Ctrl+C did not return a quit command")
	}
	if updated == nil {
		t.Fatal("TUI update returned nil model")
	}
}

func TestRunTUIRejectsNilContext(t *testing.T) {
	if err := ui.RunTUI(nil, ui.Dependencies{}); err == nil {
		t.Fatal("RunTUI(nil) succeeded; want error")
	}
}
