package ui_test

import (
	"net"
	"net/netip"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

func TestResolveTUICommandPrefersSelectionAndClonesTargets(t *testing.T) {
	state := ui.NewTUIState()
	state.SetDevices([]parser.Device{
		{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}, IPv4: netip.MustParseAddr("192.0.2.1")},
		{MAC: net.HardwareAddr{6, 7, 8, 9, 10, 11}, IPv4: netip.MustParseAddr("192.0.2.2")},
	})
	state.MoveFocus(1)
	state.ToggleFocusedSelection()
	state.MoveFocus(-1)
	state.ToggleFocusedSelection()

	resolved, err := ui.ResolveTUICommand(state, ui.ActivateCommand{
		Targets: ui.TargetSelector{UseFocusedDevice: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	command := resolved.(ui.ActivateCommand)
	if len(command.Targets.MACs) != 2 || command.Targets.UseFocusedDevice {
		t.Fatalf("resolved targets = %#v", command.Targets)
	}
}

func TestResolveTUICommandRejectsEmptyTargetSet(t *testing.T) {
	_, err := ui.ResolveTUICommand(ui.NewTUIState(), ui.NetworkCommand{
		Targets: ui.TargetSelector{UseFocusedDevice: true},
	})
	if err == nil {
		t.Fatal("empty target set was accepted")
	}
}
