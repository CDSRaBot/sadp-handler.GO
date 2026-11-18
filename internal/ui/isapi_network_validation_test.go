package ui

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func TestISAPINetworkParsing(t *testing.T) {
	base := []string{"isapi-network", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05",
		"--username", "admin", "--password-stdin", "--interface-id", "1", "--yes"}
	for _, args := range [][]string{
		append(append([]string{}, base...), "--dhcp", "--discovery-interface", "2"),
		append(append([]string{}, base...), "--new-address", "198.51.100.20/24", "--gateway", "198.51.100.1"),
	} {
		if _, err := ParseCLICommand(args); err != nil {
			t.Fatalf("valid network command rejected: %v", err)
		}
	}
	for _, args := range [][]string{
		append(append([]string{}, base...), "--dhcp"),
		append(append([]string{}, base...), "--dhcp", "--discovery-interface", "0"),
		append(append([]string{}, base...), "--dhcp", "--discovery-interface", "2", "--new-address", "198.51.100.20/24"),
		append(append([]string{}, base...), "--new-address", "198.51.100.20/24", "--gateway", "198.51.100.1", "--discovery-interface", "0"),
		append(append([]string{}, base...), "--new-address", "198.51.100.0/24", "--gateway", "198.51.100.1"),
		append(append([]string{}, base...), "--new-address", "198.51.100.20/24", "--gateway", "198.51.100.255"),
		append(append([]string{}, base...), "--new-address", "198.51.100.20/24", "--gateway", "203.0.113.1"),
		append(append([]string{}, base...), "--dhcp", "--discovery-interface", "2", "--yes"),
		{"isapi-network", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin", "--interface-id", "1", "--dhcp", "--discovery-interface", "2"},
	} {
		if _, err := ParseCLICommand(args); err == nil {
			t.Errorf("invalid network options accepted")
		}
	}
	if _, err := ParseConsoleCommand("isapi-network --address 192.0.2.1"); err == nil {
		t.Fatal("interactive network command accepted")
	}
}

type emptyNetworkInterfaces struct{}

func (emptyNetworkInterfaces) ListEligibleInterfaces() ([]adapter.Interface, error) {
	return []adapter.Interface{{Index: 3}}, nil
}

func TestISAPINetworkDHCPChecksInterfaceBeforePassword(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	readPassword := false
	service := ISAPIKnownAddressNetworkService{
		Passwords: passwordProviderFunc(func(context.Context) (string, error) {
			readPassword = true
			return "synthetic-password", nil
		}),
		Profile:    syntheticNetworkProfile{},
		Gate:       syntheticNetworkGate(func(context.Context, route.Target, route.DeviceInfo) error { return nil }),
		Sessions:   snapshotSessions{},
		Interfaces: emptyNetworkInterfaces{},
		Policy:     DiscoveryPolicy{ProbeInterval: time.Second, MaxSADPDatagramSize: 1500},
	}
	command := ISAPINetworkCommand{
		Connection: ISAPIDiscoverCommand{
			Address: netip.MustParseAddr("127.0.0.1"), MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5},
			Port: uint16(listener.Addr().(*net.TCPAddr).Port), Username: "admin", PasswordInput: PasswordFromStdin,
		},
		InterfaceID: "1", Plan: logic.NetworkConfig{UseDHCP: true}, DiscoveryInterface: 2, Confirm: true,
	}
	result, err := service.ConfigureISAPINetwork(context.Background(), command)
	var routeErr *route.Error
	if !errors.As(err, &routeErr) || routeErr.Kind != route.ErrorInvalidInput || result.Attempted || readPassword {
		t.Fatalf("result=%+v err=%v readPassword=%v", result, err, readPassword)
	}
}

func TestISAPINetworkRejectsInvalidDeviceGate(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/ISAPI/System/deviceInfo" {
			t.Error("network request sent after identity gate failure")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress></DeviceInfo>"))
	}))
	defer server.Close()
	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	service := ISAPIKnownAddressNetworkService{
		Passwords: passwordProviderFunc(func(context.Context) (string, error) { return "synthetic-password", nil }),
		Profile:   syntheticNetworkProfile{},
		Gate: syntheticNetworkGate(func(_ context.Context, target route.Target, _ route.DeviceInfo) error {
			target.MAC[0] = 0xff
			return nil
		}),
	}
	result, err := service.ConfigureISAPINetwork(context.Background(), ISAPINetworkCommand{
		Connection: ISAPIDiscoverCommand{Address: netip.MustParseAddr(host), MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5},
			Port: uint16(port), Username: "admin", PasswordInput: PasswordFromStdin},
		InterfaceID: "1", Plan: logic.NetworkConfig{Address: netip.MustParsePrefix("198.51.100.20/24"),
			Gateway: netip.MustParseAddr("198.51.100.1")}, Confirm: true,
	})
	var routeErr *route.Error
	if !errors.As(err, &routeErr) || routeErr.Kind != route.ErrorVerification || result.Attempted || requests.Load() != 2 {
		t.Fatalf("result=%+v err=%v requests=%d", result, err, requests.Load())
	}
}
