package ui

import (
	"context"
	"testing"
)

func TestTUIModelAggregatesRepeatedDiscoveryEventsByMACAndSender(t *testing.T) {
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{})
	event := DiscoveredDevice{
		MAC:    "00:11:22:33:44:55",
		Sender: "192.0.2.1:37020",
		IPv4:   "192.0.2.20",
		Model:  "Alpha",
	}
	model.applyDiscoveryEvent(event)
	event.Model = "Alpha-Updated"
	model.applyDiscoveryEvent(event)

	if len(model.discovered) != 1 {
		t.Fatalf("discovered count = %d, want 1", len(model.discovered))
	}
	if model.discovered[0].Model != "Alpha-Updated" {
		t.Fatalf("device was not updated: %#v", model.discovered[0])
	}
	if len(model.state.VisibleDevices()) != 1 {
		t.Fatalf("state device count = %d, want 1", len(model.state.VisibleDevices()))
	}
}

func TestTUIModelRejectsUnsafeDiscoveryIdentity(t *testing.T) {
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{})
	model.applyDiscoveryEvent(DiscoveredDevice{
		MAC:    "invalid",
		Sender: "192.0.2.1",
		IPv4:   "192.0.2.20",
	})
	if len(model.discovered) != 0 || model.status != "received invalid discovery event" {
		t.Fatalf("invalid event was accepted: %#v status=%q", model.discovered, model.status)
	}
}
