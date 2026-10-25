package ui

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTUIFilterUpdatesIncrementallyWithoutClearingSelection(t *testing.T) {
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{})
	model.SetDevices([]parser.Device{
		{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}, IPv4: netip.MustParseAddr("192.0.2.1"), Model: "Alpha"},
		{MAC: net.HardwareAddr{6, 7, 8, 9, 10, 11}, IPv4: netip.MustParseAddr("192.0.2.2"), Model: "Beta"},
	})
	model.state.MoveFocus(1)
	model.state.ToggleFocusedSelection()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(TUIModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = updated.(TUIModel)

	if model.state.Filter() != "a" || len(model.state.VisibleDevices()) != 2 || model.state.SelectionCount() != 1 {
		t.Fatalf("filter state=%q visible=%d selected=%d", model.state.Filter(), len(model.state.VisibleDevices()), model.state.SelectionCount())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(TUIModel)
	if model.state.Filter() != "" || model.state.SelectionCount() != 1 {
		t.Fatalf("clearing filter changed selection: filter=%q selected=%d", model.state.Filter(), model.state.SelectionCount())
	}
}
