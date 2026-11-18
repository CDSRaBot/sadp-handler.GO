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

func TestDiscoveryIgnoresInquiriesButReportsOtherErrors(t *testing.T) {
	iface := adapter.Interface{Index: 2, Name: "synthetic", Flags: net.FlagUp,
		IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")}}
	for _, tt := range []struct {
		kind    parser.ParseErrorKind
		wantErr bool
	}{
		{parser.ParseInquiry, false},
		{parser.ParseUnsupported, true},
		{parser.ParseMalformedXML, true},
	} {
		service := M6DiscoverySessionService{
			ListInterfaces: func() ([]adapter.Interface, error) { return []adapter.Interface{iface}, nil },
			Discover: func(ctx context.Context, _ adapter.Interface, _ discovery.DiscoveryOptions, handler discovery.EventHandler) error {
				return handler(ctx, discovery.Event{Kind: discovery.DatagramRejected, Err: &parser.ParseError{
					Kind: tt.kind, Err: errors.New("synthetic"),
				}})
			},
			Multicast: adapter.MulticastOptions{Group: netip.MustParseAddrPort("239.255.255.250:37020"), TTL: 1},
		}
		index := 2
		session, err := service.Start(context.Background(), &index, DiscoveryPolicy{CycleTimeout: 1, RepeatCount: 1, MaxSADPDatagramSize: 1500})
		if err != nil {
			t.Fatal(err)
		}
		for range session.Events() {
		}
		err, ok := <-session.Errors()
		if ok != tt.wantErr || (ok && err == nil) {
			t.Fatalf("kind=%s error=%v present=%t", tt.kind, err, ok)
		}
	}
}
