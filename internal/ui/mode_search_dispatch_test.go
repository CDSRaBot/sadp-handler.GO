package ui_test

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

func TestSelectUIModeValidatesExplicitModeAndTerminalRequirements(t *testing.T) {
	testCases := []struct {
		name      string
		value     string
		once      bool
		stdinTTY  bool
		stdoutTTY bool
		want      ui.InterfaceMode
		wantErr   bool
	}{
		{name: "TUI", value: "yes", stdinTTY: true, stdoutTTY: true, want: ui.ModeTUI},
		{name: "TUI requires stdin", value: "yes", stdinTTY: false, stdoutTTY: true, wantErr: true},
		{name: "TUI requires stdout", value: "yes", stdinTTY: true, stdoutTTY: false, wantErr: true},
		{name: "TUI once conflict", value: "yes", once: true, stdinTTY: true, stdoutTTY: true, wantErr: true},
		{name: "interactive CLI", value: "no", stdinTTY: true, stdoutTTY: false, want: ui.ModeCLIInteractive},
		{name: "CLI requires stdin", value: "no", stdinTTY: false, wantErr: true},
		{name: "one shot redirected", value: "no", once: true, want: ui.ModeCLIOnce},
		{name: "invalid value", value: "maybe", stdinTTY: true, stdoutTTY: true, wantErr: true},
		{name: "empty value", value: "", stdinTTY: true, stdoutTTY: true, wantErr: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ui.SelectUIMode(testCase.value, testCase.once, testCase.stdinTTY, testCase.stdoutTTY)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("SelectUIMode error = %v, want error=%t", err, testCase.wantErr)
			}
			if !testCase.wantErr && got != testCase.want {
				t.Fatalf("mode = %q, want %q", got, testCase.want)
			}
			if testCase.wantErr && got != "" {
				t.Fatalf("mode = %q on error, want empty", got)
			}
		})
	}
}

func TestSearchDevicesFiltersFieldsAndPreservesInput(t *testing.T) {
	sender := &net.UDPAddr{IP: netip.MustParseAddr("192.0.2.1").AsSlice(), Port: 37020}
	devices := []parser.Device{
		{MAC: net.HardwareAddr{0, 17, 34, 51, 68, 85}, IPv4: netip.MustParseAddr("192.0.2.20"), Model: "Alpha", SerialNumber: "SN-A", Activated: true, Sender: sender},
		{MAC: net.HardwareAddr{10, 11, 12, 13, 14, 15}, IPv4: netip.MustParseAddr("192.0.2.21"), Model: "Beta", SerialNumber: "SN-B", Activated: false, Sender: sender},
	}
	for _, query := range []string{"", "  ", "alpha", "sn-b", "192.0.2.21", "0a:0b:0c"} {
		t.Run(query, func(t *testing.T) {
			got := ui.SearchDevices(devices, query)
			if got == nil {
				t.Fatalf("SearchDevices(%q) returned nil; want a slice", query)
			}
			if query == "" || query == "  " {
				if len(got) != 2 {
					t.Fatalf("empty query result length = %d, want 2", len(got))
				}
			} else if len(got) != 1 {
				t.Fatalf("query result = %#v, want one match", got)
			}
		})
	}
	if devices[0].Model != "Alpha" || devices[1].Activated != false {
		t.Fatal("SearchDevices mutated the input snapshot")
	}
}

func TestSearchDevicesSortsInactiveBeforeActiveStably(t *testing.T) {
	devices := []parser.Device{
		{Model: "active", Activated: true},
		{Model: "inactive-a", Activated: false},
		{Model: "inactive-b", Activated: false},
	}
	got := ui.SearchDevices(devices, "")
	if len(got) != 3 || got[0].Model != "inactive-a" || got[1].Model != "inactive-b" || got[2].Model != "active" {
		t.Fatalf("ordered devices = %#v", got)
	}
}

func TestDispatchCommandRejectsCanceledContextAndReturnsSafeResultShape(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := ui.DispatchCommand(ctx, ui.HelpCommand{}, ui.Dependencies{})
	if err == nil || result.OK {
		t.Fatalf("DispatchCommand = %#v, %v; want cancellation failure", result, err)
	}
}

func TestCommandContractsExposeExplicitPolicyAndFailureValues(t *testing.T) {
	policy := ui.DiscoveryPolicy{
		ProbeInterval:       time.Minute,
		MaxSADPDatagramSize: 1500,
	}
	if policy.ProbeInterval <= 0 || policy.MaxSADPDatagramSize != 1500 {
		t.Fatalf("unexpected discovery policy: %#v", policy)
	}
	if ui.FailureInvalidArgument == "" || ui.FailureUnsupported == "" || ui.FailureOperationFailed == "" || ui.FailureCanceled == "" {
		t.Fatal("failure code contract is incomplete")
	}
}
