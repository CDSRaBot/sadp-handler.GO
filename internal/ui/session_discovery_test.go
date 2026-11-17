package ui

import (
	"context"
	"errors"
	"testing"
	"time"
)

type snapshotSession struct {
	events chan DiscoveredDevice
	errs   chan error
}

func (s snapshotSession) Events() <-chan DiscoveredDevice { return s.events }
func (s snapshotSession) Errors() <-chan error            { return s.errs }

type snapshotSessions struct {
	session DiscoverySession
	err     error
}

func (s snapshotSessions) Start(context.Context, *int, DiscoveryPolicy) (DiscoverySession, error) {
	return s.session, s.err
}

func TestSessionDiscoveryCollectsAndSortsSnapshot(t *testing.T) {
	events := make(chan DiscoveredDevice, 4)
	errs := make(chan error)
	seen := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events <- DiscoveredDevice{MAC: "AA:BB:CC:DD:EE:FF", Sender: "192.0.2.2:37020", IPv4: "192.0.2.20"}
	events <- DiscoveredDevice{MAC: "00:11:22:33:44:55", Sender: "192.0.2.3:37020", IPv4: "192.0.2.30"}
	events <- DiscoveredDevice{MAC: "aa:bb:cc:dd:ee:ff", Sender: "192.0.2.2:40000", IPv4: "192.0.2.21", LastSeen: seen}
	events <- DiscoveredDevice{MAC: "aa:bb:cc:dd:ee:ff", Sender: "192.0.2.4:37020", IPv4: "192.0.2.40"}
	close(events)
	close(errs)
	service := SessionDiscoveryService{Sessions: snapshotSessions{session: snapshotSession{events, errs}}}
	result, err := service.Discover(context.Background(), nil, DiscoveryPolicy{CycleTimeout: time.Second, RepeatCount: 1, MaxSADPDatagramSize: 1500})
	if err != nil || len(result) != 3 || result[0].MAC != "00:11:22:33:44:55" ||
		result[1].IPv4 != "192.0.2.21" || !result[1].LastSeen.Equal(seen) || result[2].IPv4 != "192.0.2.40" {
		t.Fatalf("snapshot = %#v, error = %v", result, err)
	}
}

func TestSessionDiscoveryReportsPartialFailure(t *testing.T) {
	events := make(chan DiscoveredDevice, 1)
	errs := make(chan error, 1)
	events <- DiscoveredDevice{MAC: "00:11:22:33:44:55", Sender: "192.0.2.2:37020", IPv4: "192.0.2.20"}
	cause := errors.New("synthetic discovery error")
	errs <- cause
	close(events)
	close(errs)
	service := SessionDiscoveryService{Sessions: snapshotSessions{session: snapshotSession{events, errs}}}
	policy := DiscoveryPolicy{CycleTimeout: time.Second, RepeatCount: 1, MaxSADPDatagramSize: 1500}
	result, err := DispatchCommand(context.Background(), DiscoverCommand{Policy: policy}, Dependencies{Discovery: service})
	if !errors.Is(err, cause) || result.OK || result.Error == nil || len(result.Discovery) != 1 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestSessionDiscoveryRejectsUnboundedAndHonorsCancellation(t *testing.T) {
	service := SessionDiscoveryService{Sessions: snapshotSessions{session: snapshotSession{make(chan DiscoveredDevice), make(chan error)}}}
	if _, err := service.Discover(context.Background(), nil, DiscoveryPolicy{ProbeInterval: time.Second, MaxSADPDatagramSize: 1500}); err == nil {
		t.Fatal("continuous discovery was accepted for a snapshot")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.Discover(ctx, nil, DiscoveryPolicy{CycleTimeout: time.Second, RepeatCount: 1, MaxSADPDatagramSize: 1500})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
}
