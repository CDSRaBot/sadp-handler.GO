package ui_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

func TestTUIInterfaceSelectionFiltersAndRequiresExplicitEligibleIndex(t *testing.T) {
	model := ui.NewTUIModel(context.Background(), ui.NewTUIState(), ui.Dependencies{})
	model.SetAvailableInterfaces([]adapter.Interface{
		{Index: 4, Name: "down", Flags: net.FlagLoopback, IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.4/24")}},
		{Index: 3, Name: "eth1", Flags: net.FlagUp, IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.3/24")}},
		{Index: 2, Name: "eth0", Flags: net.FlagUp, IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.2/24")}},
	})
	interfaces := model.AvailableInterfaces()
	if len(interfaces) != 2 || interfaces[0].Index != 2 || interfaces[1].Index != 3 {
		t.Fatalf("eligible interfaces = %#v", interfaces)
	}
	if err := model.SelectInterface(4); err == nil {
		t.Fatal("ineligible interface was selected")
	}
	if err := model.SelectInterface(3); err != nil {
		t.Fatal(err)
	}
	selected, ok := model.SelectedInterface()
	if !ok || selected.Index != 3 {
		t.Fatalf("selected interface = %#v, %t", selected, ok)
	}
}
