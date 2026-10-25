package ui_test

import (
	"net"
	"net/netip"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

func TestTUIStatePreservesSelectionAcrossFilteringAndRefresh(t *testing.T) {
	devices := []parser.Device{
		{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}, IPv4: netip.MustParseAddr("192.0.2.1"), Model: "Alpha"},
		{MAC: net.HardwareAddr{6, 7, 8, 9, 10, 11}, IPv4: netip.MustParseAddr("192.0.2.2"), Model: "Beta"},
	}
	state := ui.NewTUIState()
	state.SetDevices(devices)
	state.MoveFocus(1)
	if !state.ToggleFocusedSelection() || state.SelectionCount() != 1 {
		t.Fatal("expected focused device to be selected")
	}
	state.SetFilter("alpha")
	if len(state.VisibleDevices()) != 1 || state.SelectionCount() != 1 {
		t.Fatalf("filter lost selection: visible=%d selected=%d", len(state.VisibleDevices()), state.SelectionCount())
	}
	state.SetDevices([]parser.Device{devices[1], devices[0]})
	selected := state.SelectedDevices()
	if len(selected) != 1 || selected[0].Model != "Beta" {
		t.Fatalf("selection was not attached to stable identity: %#v", selected)
	}
}

func TestTUIStateHistoryAndDisplaySanitization(t *testing.T) {
	state := ui.NewTUIState()
	if err := state.AddHistory("version"); err != nil {
		t.Fatal(err)
	}
	if err := state.AddHistory("activate --password secret"); err == nil {
		t.Fatal("sensitive command was accepted into history")
	}
	if got, ok := state.PreviousHistory(); !ok || got != "version" {
		t.Fatalf("previous history = %q, %t", got, ok)
	}
	state.SetDetail("device\x1b[31m\nsummary")
	if state.Detail() != "device [31m\nsummary" {
		t.Fatalf("detail was not sanitized: %q", state.Detail())
	}
}
