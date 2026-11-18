package ui

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type interactionDiscovery struct {
	contexts []context.Context
	indices  []int
	policies []DiscoveryPolicy
}

func (s *interactionDiscovery) Start(ctx context.Context, index *int, policy DiscoveryPolicy) (DiscoverySession, error) {
	s.contexts = append(s.contexts, ctx)
	s.indices = append(s.indices, *index)
	s.policies = append(s.policies, policy)
	session := snapshotSession{events: make(chan DiscoveredDevice), errs: make(chan error)}
	go func() {
		<-ctx.Done()
		close(session.events)
		close(session.errs)
	}()
	return session, nil
}

func interactionKey(model TUIModel, key tea.KeyMsg) (TUIModel, tea.Cmd) {
	updated, command := model.Update(key)
	return updated.(TUIModel), command
}

func TestTUIInterfacePickerAndSessionCancellation(t *testing.T) {
	service := &interactionDiscovery{}
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{
		DiscoverySession: service,
		DiscoveryPolicy:  DiscoveryPolicy{CycleTimeout: 5 * time.Second, RepeatCount: 1, MaxSADPDatagramSize: 65507},
	})
	model.SetAvailableInterfaces([]adapter.Interface{
		{Index: 2, Name: "first", Flags: net.FlagUp, IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")}},
		{Index: 8, Name: "second", Flags: net.FlagUp, IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("198.51.100.1/24")}},
	})
	model, _ = interactionKeyFromMessage(model, tea.WindowSizeMsg{Width: 100, Height: 30})
	model, cmd := interactionKey(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if !model.interfacePicker || cmd != nil || len(service.indices) != 0 ||
		!strings.Contains(model.View(), "first") || !strings.Contains(model.View(), "second") {
		t.Fatal("interface picker did not open without starting discovery")
	}
	model, _ = interactionKey(model, tea.KeyMsg{Type: tea.KeyDown})
	model, cmd = interactionKey(model, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || model.interfacePicker || len(service.indices) != 0 {
		t.Fatal("selection did not defer discovery to the command")
	}
	message := cmd().(tuiDiscoverySessionMsg)
	model, _ = interactionKeyFromMessage(model, message)
	if len(service.indices) != 1 || service.indices[0] != 8 || service.policies[0].ProbeInterval != time.Minute ||
		service.policies[0].CycleTimeout != 0 || service.policies[0].MaxSADPDatagramSize != 65507 {
		t.Fatalf("unexpected discovery inputs: %+v %+v", service.indices, service.policies)
	}
	oldGeneration := model.discoveryGeneration
	model, _ = interactionKey(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	model, _ = interactionKey(model, tea.KeyMsg{Type: tea.KeyUp})
	model, cmd = interactionKey(model, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || service.contexts[0].Err() != context.Canceled {
		t.Fatal("previous discovery was not canceled")
	}
	message = cmd().(tuiDiscoverySessionMsg)
	model, _ = interactionKeyFromMessage(model, message)
	if len(service.indices) != 2 || service.indices[1] != 2 {
		t.Fatalf("wrong selected interface: %v", service.indices)
	}
	model, _ = interactionKeyFromMessage(model, tuiDiscoveryEventMsg{
		generation: oldGeneration, device: DiscoveredDevice{MAC: "00:01:02:03:04:05"},
	})
	if len(model.discovered) != 0 {
		t.Fatal("stale discovery event changed current interface results")
	}
	model.stopDiscovery()
	if service.contexts[1].Err() != context.Canceled {
		t.Fatal("current discovery was not canceled")
	}
}

func interactionKeyFromMessage(model TUIModel, message tea.Msg) (TUIModel, tea.Cmd) {
	updated, command := model.Update(message)
	return updated.(TUIModel), command
}

func TestTUINavigationSelectionDetailsAndZones(t *testing.T) {
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{})
	model.SetDevices([]parser.Device{
		{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}, IPv4: netip.MustParseAddr("192.0.2.1"), Model: "First"},
		{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 6}, IPv4: netip.MustParseAddr("192.0.2.2"), Model: "Second", SerialNumber: "Synthetic-SN"},
	})
	model, _ = interactionKeyFromMessage(model, tea.WindowSizeMsg{Width: 100, Height: 30})
	model, _ = interactionKey(model, tea.KeyMsg{Type: tea.KeyDown})
	if model.devices.Index() != 1 {
		t.Fatalf("visible cursor did not move: %d", model.devices.Index())
	}
	model, _ = interactionKey(model, tea.KeyMsg{Type: tea.KeySpace})
	view := model.View()
	if !strings.Contains(view, ">[x]") || !strings.Contains(view, "Synthetic-SN") ||
		!strings.Contains(view, "Devices [active]") || !strings.Contains(view, "│") {
		t.Fatalf("missing selection, detail or border: %q", view)
	}
	if lipgloss.Height(view) > 30 || lipgloss.Width(view) > 100 {
		t.Fatalf("view exceeds terminal: %dx%d", lipgloss.Width(view), lipgloss.Height(view))
	}
	model, _ = interactionKey(model, tea.KeyMsg{Type: tea.KeyTab})
	if model.state.Zone() != TUIZoneDetails || !strings.Contains(model.View(), "Details [active]") {
		t.Fatal("Tab did not activate detail zone")
	}
	model, _ = interactionKey(model, tea.KeyMsg{Type: tea.KeyTab})
	if model.state.Zone() != TUIZoneConsole || !model.console.Focused() ||
		!strings.Contains(model.View(), "Console [active]") {
		t.Fatal("console zone did not receive focus")
	}
	model.console.SetValue("version")
	model, cmd := interactionKey(model, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("console did not submit command")
	}
	model, _ = interactionKeyFromMessage(model, cmd())
	if model.status != "dev" {
		t.Fatalf("console result not displayed: %q", model.status)
	}
	model, _ = interactionKey(model, tea.KeyMsg{Type: tea.KeyUp})
	if model.console.Value() != "version" {
		t.Fatal("console history did not navigate")
	}
	model.console.SetValue("quit")
	model, cmd = interactionKey(model, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("quit did not dispatch")
	}
	_, quit := interactionKeyFromMessage(model, cmd())
	if quit == nil {
		t.Fatal("quit result did not exit TUI")
	}
}

func TestTUIAndCommandEventsAreLoggedWithoutDeviceData(t *testing.T) {
	var log bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&log, nil))
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{Logger: logger})
	model.applyDiscoveryEvent(DiscoveredDevice{
		MAC: "00:01:02:03:04:05", IPv4: "192.0.2.1", Sender: "192.0.2.1",
		Model: "private-model", SerialNumber: "private-serial",
	})
	_, err := DispatchCommand(context.Background(), VersionCommand{}, Dependencies{Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	text := log.String()
	for _, expected := range []string{"ui.device_discovered", "ui.command_started", "ui.command_completed"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing log event %q: %q", expected, text)
		}
	}
	for _, secret := range []string{"private-model", "private-serial", "192.0.2.1", "00:01:02:03:04:05"} {
		if strings.Contains(text, secret) {
			t.Fatalf("device data leaked in log: %q", text)
		}
	}
}
