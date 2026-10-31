package ui

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/discovery"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

func TestM6DiscoverySessionResolvesInterfaceAndStreamsDevices(t *testing.T) {
	iface := adapter.Interface{
		Index: 2, Name: "eth0", Flags: net.FlagUp,
		IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.10/24")},
	}
	service := M6DiscoverySessionService{
		ListInterfaces: func() ([]adapter.Interface, error) { return []adapter.Interface{iface}, nil },
		Discover: func(ctx context.Context, got adapter.Interface, options discovery.DiscoveryOptions, handle discovery.EventHandler) error {
			if got.Index != 2 || options.MaxSADPDatagramSize != 1500 {
				t.Fatalf("unexpected discovery inputs: %#v %#v", got, options)
			}
			return handle(ctx, discovery.Event{
				Kind: discovery.DeviceFound,
				Device: &parser.Device{
					MAC:    net.HardwareAddr{0, 17, 34, 51, 68, 85},
					IPv4:   netip.MustParseAddr("192.0.2.20"),
					Sender: &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 37020},
				},
			})
		},
		Multicast: adapter.MulticastOptions{Group: netip.MustParseAddrPort("239.255.255.250:37020"), TTL: 1},
	}
	session, err := service.Start(context.Background(), intPointer(2), DiscoveryPolicy{
		CycleTimeout: 1, RepeatCount: 1, MaxSADPDatagramSize: 1500,
	})
	if err != nil {
		t.Fatal(err)
	}
	device, ok := <-session.Events()
	if !ok || device.MAC != "00:11:22:33:44:55" || device.Sender != "192.0.2.1:37020" {
		t.Fatalf("unexpected streamed device: %#v, %t", device, ok)
	}
}

func TestM6DiscoverySessionRejectsMissingOrIneligibleInterface(t *testing.T) {
	service := M6DiscoverySessionService{
		ListInterfaces: func() ([]adapter.Interface, error) { return nil, nil },
		Discover: func(context.Context, adapter.Interface, discovery.DiscoveryOptions, discovery.EventHandler) error {
			return nil
		},
		Multicast: adapter.MulticastOptions{Group: netip.MustParseAddrPort("239.255.255.250:37020"), TTL: 1},
	}
	if _, err := service.Start(context.Background(), nil, DiscoveryPolicy{ProbeInterval: 1, MaxSADPDatagramSize: 1500}); err == nil {
		t.Fatal("missing interface index was accepted")
	}
	if _, err := service.Start(context.Background(), intPointer(2), DiscoveryPolicy{ProbeInterval: 1, MaxSADPDatagramSize: 1500}); err == nil {
		t.Fatal("unavailable interface was accepted")
	}
}

func TestM6DiscoverySessionReportsRejectedDatagrams(t *testing.T) {
	iface := adapter.Interface{
		Index: 2, Name: "eth0", Flags: net.FlagUp,
		IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.10/24")},
	}
	cause := errors.New("malformed advertisement")
	service := M6DiscoverySessionService{
		ListInterfaces: func() ([]adapter.Interface, error) { return []adapter.Interface{iface}, nil },
		Discover: func(ctx context.Context, got adapter.Interface, options discovery.DiscoveryOptions, handle discovery.EventHandler) error {
			return handle(ctx, discovery.Event{
				Kind: discovery.DatagramRejected,
				Err:  cause,
			})
		},
		Multicast: adapter.MulticastOptions{Group: netip.MustParseAddrPort("239.255.255.250:37020"), TTL: 1},
	}
	session, err := service.Start(context.Background(), intPointer(2), DiscoveryPolicy{
		CycleTimeout: 1, RepeatCount: 1, MaxSADPDatagramSize: 1500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-session.Errors(); !errors.Is(err, cause) {
		t.Fatalf("session error = %v, want rejected datagram cause", err)
	}
}

func intPointer(value int) *int {
	return &value
}
