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

	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func TestISAPIActivationRejectsInvalidOptions(t *testing.T) {
	for _, args := range [][]string{
		{"isapi-activate"},
		{"isapi-activate", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05", "--username", "admin"},
		{"isapi-activate", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin", "--basic"},
		{"isapi-activate", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05", "--username", "admin", "--password", "not-allowed"},
	} {
		if _, err := ParseCLICommand(args); err == nil {
			t.Errorf("accepted invalid activation options")
		}
	}
}

func TestISAPIActivationRechecksGateAfterPassword(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
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
	gateCalls := 0
	service := ISAPIKnownAddressActivationService{
		Passwords: passwordProviderFunc(func(context.Context) (string, error) { return "SyntheticPass1!", nil }),
		Gate: isapiActivationGateFunc(func(context.Context, route.Target) error {
			gateCalls++
			if gateCalls == 2 {
				return errors.New("identity changed")
			}
			return nil
		}),
	}
	result, err := service.ActivateISAPI(context.Background(), ISAPIActivateCommand{
		Address: netip.MustParseAddr(host), MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5},
		Port: uint16(port), Username: "admin", PasswordInput: PasswordFromStdin,
	})
	var routeErr *route.Error
	if !errors.As(err, &routeErr) || routeErr.Kind != route.ErrorVerification || result.Attempted ||
		result.Verified || gateCalls != 2 || requests.Load() != 0 {
		t.Fatalf("result=%+v err=%v gate calls=%d requests=%d", result, err, gateCalls, requests.Load())
	}
}

func TestISAPIActivationRejectsGateChangingTargetBeforePassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("unexpected HTTP request")
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
	readPassword := false
	service := ISAPIKnownAddressActivationService{
		Passwords: passwordProviderFunc(func(context.Context) (string, error) {
			readPassword = true
			return "SyntheticPass1!", nil
		}),
		Gate: isapiActivationGateFunc(func(_ context.Context, target route.Target) error {
			target.MAC[0] = 0xff
			return nil
		}),
	}
	result, err := service.ActivateISAPI(context.Background(), ISAPIActivateCommand{
		Address: netip.MustParseAddr(host), MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5},
		Port: uint16(port), Username: "admin", PasswordInput: PasswordFromStdin,
	})
	var routeErr *route.Error
	if !errors.As(err, &routeErr) || routeErr.Kind != route.ErrorVerification || result.Attempted || readPassword {
		t.Fatalf("result=%+v err=%v readPassword=%v", result, err, readPassword)
	}
}
