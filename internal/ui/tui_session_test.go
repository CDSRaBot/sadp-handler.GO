package ui

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
)

type blockingDiscovery struct {
	called chan int
	first  snapshotSession
	second snapshotSession
}

func (s *blockingDiscovery) Start(_ context.Context, index *int, _ DiscoveryPolicy) (DiscoverySession, error) {
	s.called <- *index
	if *index == 1 {
		return s.first, nil
	}
	return s.second, nil
}

func TestTUIWaitsForPreviousSessionBeforeOpeningAnother(t *testing.T) {
	service := &blockingDiscovery{
		called: make(chan int, 2),
		first:  snapshotSession{events: make(chan DiscoveredDevice), errs: make(chan error)},
		second: snapshotSession{events: make(chan DiscoveredDevice), errs: make(chan error)},
	}
	model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{
		DiscoverySession: service, DiscoveryPolicy: DiscoveryPolicy{MaxSADPDatagramSize: 1500},
	})
	model.SetAvailableInterfaces([]adapter.Interface{
		{Index: 1, Name: "first", Flags: net.FlagUp, IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")}},
		{Index: 2, Name: "second", Flags: net.FlagUp, IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.2/24")}},
	})
	if err := model.SelectInterface(1); err != nil {
		t.Fatal(err)
	}
	first := model.startDiscovery()
	if first == nil {
		t.Fatal("first discovery command is missing")
	}
	_, _ = interactionKeyFromMessage(model, first())
	if index := <-service.called; index != 1 {
		t.Fatalf("first interface=%d", index)
	}
	if err := model.SelectInterface(2); err != nil {
		t.Fatal(err)
	}
	second := model.startDiscovery()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		second()
	}()
	select {
	case index := <-service.called:
		t.Fatalf("interface %d opened before old session closed", index)
	case <-time.After(100 * time.Millisecond):
	}
	close(service.first.events)
	close(service.first.errs)
	select {
	case index := <-service.called:
		if index != 2 {
			t.Fatalf("second interface=%d", index)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second session did not start after old session closed")
	}
	<-finished
	model.stopDiscovery()
	close(service.second.events)
	close(service.second.errs)
	select {
	case <-model.discoveryDone:
	case <-time.After(2 * time.Second):
		t.Fatal("second session did not stop")
	}
}
