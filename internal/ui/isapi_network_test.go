package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

type syntheticNetworkProfile struct{ restart bool }

func (p syntheticNetworkProfile) BuildUpdate(collection, current []byte, interfaceID string, _ logic.NetworkConfig) ([]byte, error) {
	if string(collection) != "<Interfaces/>" || string(current) != "<Interface/>" || interfaceID != "1" {
		return nil, errors.New("unexpected synthetic network profile data")
	}
	return []byte("<Update/>"), nil
}

func (p syntheticNetworkProfile) RestartRequired([]byte) (bool, error) { return p.restart, nil }

func (p syntheticNetworkProfile) Verify(current []byte, _ logic.NetworkConfig) error {
	if string(current) != "<Interface/>" {
		return errors.New("unexpected synthetic verification data")
	}
	return nil
}

type syntheticNetworkGate func(context.Context, route.Target, route.DeviceInfo) error

func (g syntheticNetworkGate) VerifyNetworkDevice(ctx context.Context, target route.Target, info route.DeviceInfo) error {
	return g(ctx, target, info)
}

type syntheticNetworkInterfaces struct{}

func (syntheticNetworkInterfaces) ListEligibleInterfaces() ([]adapter.Interface, error) {
	return []adapter.Interface{{Index: 2}}, nil
}

func TestISAPINetworkOneShotPipeline(t *testing.T) {
	for _, tt := range []struct {
		name         string
		profile      bool
		denyGate     bool
		dhcp         bool
		restart      bool
		disconnect   bool
		wrongMAC     bool
		wantExit     int
		wantCode     string
		attempted    bool
		verified     bool
		readPassword bool
	}{
		{name: "missing profile", wantExit: ExitOperational, wantCode: "unsupported"},
		{name: "unverified device", profile: true, denyGate: true, wantExit: ExitOperational, wantCode: "unsupported", readPassword: true},
		{name: "static verified", profile: true, wantExit: ExitSuccess, attempted: true, verified: true, readPassword: true},
		{name: "DHCP verified", profile: true, dhcp: true, wantExit: ExitSuccess, attempted: true, verified: true, readPassword: true},
		{name: "restart required", profile: true, restart: true, wantExit: ExitOperational, wantCode: "verification", attempted: true, readPassword: true},
		{name: "uncertain delivery", profile: true, disconnect: true, wantExit: ExitOperational, wantCode: "transport", attempted: true, readPassword: true},
		{name: "identity changed", profile: true, wrongMAC: true, wantExit: ExitOperational, wantCode: "verification", attempted: true, readPassword: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requests, puts atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				user, password, ok := r.BasicAuth()
				if !ok || user != "admin" || password != "synthetic-password" {
					w.Header().Set("WWW-Authenticate", `Basic realm="synthetic"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.Method == http.MethodPut {
					puts.Add(1)
					if r.URL.Path != "/ISAPI/System/Network/interfaces/1" {
						t.Error("unexpected mutation path")
					}
					if tt.disconnect {
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = conn.Close()
						return
					}
					_, _ = fmt.Fprint(w, "<ResponseStatus><statusCode>1</statusCode></ResponseStatus>")
					return
				}
				if r.Method != http.MethodGet {
					t.Error("unexpected method")
					return
				}
				switch r.URL.Path {
				case "/ISAPI/System/deviceInfo":
					mac := "00:01:02:03:04:05"
					if tt.wrongMAC && puts.Load() != 0 {
						mac = "00:01:02:03:04:06"
					}
					_, _ = fmt.Fprintf(w, "<DeviceInfo><macAddress>%s</macAddress><model>Synthetic</model></DeviceInfo>", mac)
				case "/ISAPI/System/Network/interfaces":
					_, _ = fmt.Fprint(w, "<Interfaces/>")
				case "/ISAPI/System/Network/interfaces/1":
					_, _ = fmt.Fprint(w, "<Interface/>")
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			host, port, err := net.SplitHostPort(server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			clientTransport := server.Client().Transport.(*http.Transport).Clone()
			clientTransport.TLSClientConfig.ServerName = host
			clientTransport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
			}
			defer clientTransport.CloseIdleConnections()
			events := make(chan DiscoveredDevice, 1)
			errs := make(chan error)
			events <- DiscoveredDevice{MAC: "00:01:02:03:04:05", IPv4: "198.51.100.20"}
			close(events)
			close(errs)
			readPassword := false
			service := ISAPIKnownAddressNetworkService{
				Client: &http.Client{Transport: clientTransport},
				Passwords: passwordProviderFunc(func(context.Context) (string, error) {
					readPassword = true
					return "synthetic-password", nil
				}),
				Sessions:   snapshotSessions{session: snapshotSession{events, errs}},
				Interfaces: syntheticNetworkInterfaces{},
				Policy:     DiscoveryPolicy{ProbeInterval: time.Second, MaxSADPDatagramSize: 1500},
			}
			if tt.profile {
				service.Profile = syntheticNetworkProfile{restart: tt.restart}
				service.Gate = syntheticNetworkGate(func(_ context.Context, target route.Target, info route.DeviceInfo) error {
					if tt.denyGate || target.MAC.String() != "00:01:02:03:04:05" || info.Model != "Synthetic" {
						return errors.New("synthetic-secret")
					}
					return nil
				})
			}
			args := []string{"--ui=no", "--format=json", "--once", "isapi-network", "--address", host,
				"--port", port, "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin",
				"--https", "--basic", "--interface-id", "1", "--yes"}
			if tt.dhcp {
				args = append(args, "--dhcp", "--discovery-interface", "2")
			} else {
				args = append(args, "--new-address", "198.51.100.20/24", "--gateway", "198.51.100.1")
			}
			var stdout, stderr bytes.Buffer
			exit := RunCLIWithTerminalState(context.Background(), args, strings.NewReader(""), &stdout, &stderr, false, false,
				Dependencies{ISAPINetwork: service})
			var result CommandResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("invalid output %q: %v", stdout.String(), err)
			}
			if exit != tt.wantExit || result.ISAPINetwork == nil || result.ISAPINetwork.Attempted != tt.attempted ||
				result.ISAPINetwork.Verified != tt.verified || result.ISAPINetwork.RestartRequired != tt.restart ||
				readPassword != tt.readPassword || result.OK != tt.verified || (puts.Load() != 0) != tt.attempted {
				t.Fatalf("exit=%d result=%+v password=%v puts=%d stderr=%s", exit, result, readPassword, puts.Load(), stderr.String())
			}
			if tt.wantCode != "" && (result.Error == nil || string(result.Error.Code) != tt.wantCode) {
				t.Fatalf("unexpected error: %+v", result.Error)
			}
			if tt.verified && result.ISAPINetwork.ObservedAddress != "198.51.100.20" {
				t.Fatalf("unexpected observed address: %+v", result.ISAPINetwork)
			}
			if strings.Contains(stdout.String()+stderr.String(), "synthetic-password") || strings.Contains(stdout.String()+stderr.String(), "synthetic-secret") {
				t.Fatal("secret leaked to output")
			}
			if !tt.profile && requests.Load() != 0 {
				t.Fatal("disabled command contacted device")
			}
		})
	}
}
