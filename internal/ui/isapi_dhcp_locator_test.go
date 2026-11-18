package ui

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

type countingDHCPSessions struct{ calls int }

func (s *countingDHCPSessions) Start(context.Context, *int, DiscoveryPolicy) (DiscoverySession, error) {
	s.calls++
	return nil, errors.New("unexpected discovery start")
}

type checkingDHCPSessions struct {
	t       *testing.T
	session DiscoverySession
}

func (s checkingDHCPSessions) Start(ctx context.Context, index *int, policy DiscoveryPolicy) (DiscoverySession, error) {
	s.t.Helper()
	deadline, ok := ctx.Deadline()
	if index == nil || *index != 2 || policy.ProbeInterval != time.Second ||
		!ok || time.Until(deadline) <= 0 || time.Until(deadline) > 60*time.Second {
		s.t.Fatal("DHCP discovery interface, policy or deadline is incorrect")
	}
	return s.session, nil
}

func TestISAPIDHCPLocatorAuthenticatesMatchingDiscoveryCandidate(t *testing.T) {
	tests := []struct {
		name        string
		returnedMAC string
		wantErr     route.ErrorKind
	}{
		{name: "matching identity", returnedMAC: "00:01:02:03:04:05"},
		{name: "different authenticated device", returnedMAC: "00:01:02:03:04:06", wantErr: route.ErrorVerification},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Authorization") == "" {
					writer.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = writer.Write([]byte(`<DeviceInfo><macAddress>` + tt.returnedMAC + `</macAddress></DeviceInfo>`))
			}))
			defer server.Close()
			var attempts atomic.Int32
			clientTransport := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				if attempts.Add(1) == 1 {
					return nil, errors.New("synthetic device is starting")
				}
				return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
			}}
			defer clientTransport.CloseIdleConnections()
			events := make(chan DiscoveredDevice, 3)
			errs := make(chan error)
			events <- DiscoveredDevice{MAC: "00:01:02:03:04:99", IPv4: "198.51.100.30"}
			events <- DiscoveredDevice{MAC: "00:01:02:03:04:05", IPv4: "198.51.100.20"}
			events <- DiscoveredDevice{MAC: "00:01:02:03:04:05", IPv4: "198.51.100.20"}
			close(events)
			close(errs)
			locator := ISAPIDHCPNetworkLocator{
				Sessions:       checkingDHCPSessions{t: t, session: snapshotSession{events, errs}},
				InterfaceIndex: 2,
				Policy:         DiscoveryPolicy{ProbeInterval: time.Second, MaxSADPDatagramSize: 1500},
				Transport:      &route.ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "synthetic-password", Client: &http.Client{Transport: clientTransport}},
			}

			original := route.Target{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}, Address: netip.MustParseAddr("192.0.2.10")}
			target, err := locator.Await(context.Background(), original, logic.NetworkConfig{UseDHCP: true})
			if tt.wantErr == "" {
				if err != nil || target.Address.String() != "198.51.100.20" || attempts.Load() < 2 {
					t.Fatalf("target = %#v, attempts = %d, error = %v", target, attempts.Load(), err)
				}
			} else {
				var routeErr *route.Error
				if !errors.As(err, &routeErr) || routeErr.Kind != tt.wantErr || target.Address.IsValid() {
					t.Fatalf("target = %#v, error = %v, want %s", target, err, tt.wantErr)
				}
			}
		})
	}
}

func TestISAPIDHCPLocatorRejectsInvalidOrMissingAdvertisements(t *testing.T) {
	tests := []struct {
		name   string
		device *DiscoveredDevice
		kind   route.ErrorKind
	}{
		{name: "invalid MAC", device: &DiscoveredDevice{MAC: "invalid"}, kind: route.ErrorMalformed},
		{name: "invalid address", device: &DiscoveredDevice{MAC: "00:01:02:03:04:05", IPv4: "224.0.0.1"}, kind: route.ErrorMalformed},
		{name: "no device", kind: route.ErrorVerification},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := make(chan DiscoveredDevice, 1)
			errs := make(chan error)
			if tt.device != nil {
				events <- *tt.device
			}
			close(events)
			close(errs)
			locator := ISAPIDHCPNetworkLocator{
				Sessions:       snapshotSessions{session: snapshotSession{events, errs}},
				InterfaceIndex: 2,
				Policy:         DiscoveryPolicy{ProbeInterval: time.Second, MaxSADPDatagramSize: 1500},
				Transport:      &route.ISAPIHTTPTransport{Username: "admin", Password: "synthetic-password"},
			}
			_, err := locator.Await(context.Background(), route.Target{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}}, logic.NetworkConfig{UseDHCP: true})
			var routeErr *route.Error
			if !errors.As(err, &routeErr) || routeErr.Kind != tt.kind {
				t.Fatalf("error = %v, want %s", err, tt.kind)
			}
		})
	}
}

func TestISAPIDHCPLocatorHonorsCancellationAndDiscoveryErrors(t *testing.T) {
	events := make(chan DiscoveredDevice)
	errs := make(chan error, 1)
	locator := ISAPIDHCPNetworkLocator{
		Sessions:       snapshotSessions{session: snapshotSession{events, errs}},
		InterfaceIndex: 2,
		Policy:         DiscoveryPolicy{ProbeInterval: time.Second, MaxSADPDatagramSize: 1500},
		Transport:      &route.ISAPIHTTPTransport{Username: "admin", Password: "synthetic-password"},
	}
	target := route.Target{MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	counter := &countingDHCPSessions{}
	locator.Sessions = counter
	if _, err := locator.Await(ctx, target, logic.NetworkConfig{UseDHCP: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if counter.calls != 0 {
		t.Fatal("canceled locator started discovery")
	}
	locator.Sessions = snapshotSessions{session: snapshotSession{events, errs}}
	cause := errors.New("synthetic discovery failure")
	errs <- cause
	if _, err := locator.Await(context.Background(), target, logic.NetworkConfig{UseDHCP: true}); !errors.Is(err, cause) {
		t.Fatalf("discovery error = %v", err)
	}
	if _, err := locator.Await(context.Background(), target, logic.NetworkConfig{}); err == nil {
		t.Fatal("invalid network plan was accepted")
	}
}
