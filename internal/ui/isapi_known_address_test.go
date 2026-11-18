package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

type notifiedReader struct {
	io.ReadCloser
	started chan struct{}
}

func (r notifiedReader) Read(p []byte) (int, error) {
	close(r.started)
	return r.ReadCloser.Read(p)
}

type passwordProviderFunc func(context.Context) (string, error)

func (f passwordProviderFunc) Password(ctx context.Context) (string, error) { return f(ctx) }

func TestParseISAPIDiscover(t *testing.T) {
	valid := []string{"isapi-discover", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin"}
	command, err := ParseCLICommand(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got := command.(ISAPIDiscoverCommand); got.Port != 80 || got.HTTPS || got.AllowBasic {
		t.Fatalf("unexpected defaults: %+v", got)
	}
	tests := [][]string{
		{"isapi-discover", "--address", "example.com", "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin"},
		{"isapi-discover", "--address", "fe80::1", "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin"},
		{"isapi-discover", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05", "--username", "admin"},
		append(append([]string{}, valid...), "--basic"),
		append(append([]string{}, valid...), "--port", "0"),
		append(append([]string{}, valid...), "--password-stdin"),
		append(append([]string{}, valid...), "--unknown"),
	}
	for _, args := range tests {
		if _, err := ParseCLICommand(args); err == nil {
			t.Errorf("accepted invalid command: %v", args)
		}
	}
	tlsCommand, err := ParseCLICommand(append(append([]string{}, valid...), "--https", "--basic"))
	if err != nil || tlsCommand.(ISAPIDiscoverCommand).Port != 443 {
		t.Fatalf("HTTPS defaults: %v, %v", tlsCommand, err)
	}
}

func TestOneShotPasswordInput(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		valid       bool
	}{
		{"line", "synthetic-password\r\nignored", true},
		{"EOF", "synthetic-password", true},
		{"maximum", strings.Repeat("x", maxOneShotPasswordBytes) + "\n", true},
		{"too long", strings.Repeat("x", maxOneShotPasswordBytes+1) + "\n", false},
		{"empty", "\n", false},
		{"nul", "x\x00y\n", false},
		{"invalid UTF-8", "\xff\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := oneShotPasswordProvider{reader: io.NopCloser(strings.NewReader(tt.input))}
			password, err := provider.Password(context.Background())
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v, error=%v", tt.valid, err)
			}
			if tt.valid && len(password) == 0 {
				t.Fatal("empty password")
			}
		})
	}
	provider := oneShotPasswordProvider{reader: io.NopCloser(strings.NewReader("secret")), terminal: true}
	if _, err := provider.Password(context.Background()); err == nil {
		t.Fatal("terminal input accepted")
	}
	reader, writer := io.Pipe()
	defer writer.Close()
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := (oneShotPasswordProvider{reader: notifiedReader{ReadCloser: reader, started: started}}).Password(ctx)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("password input did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected cancellation result: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("password input did not cancel")
	}
}

func TestISAPIDiscoveryChecksRouteBeforeReadingPassword(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	readPassword := false
	service := ISAPIKnownAddressService{Passwords: passwordProviderFunc(func(context.Context) (string, error) {
		readPassword = true
		return "synthetic-password", nil
	})}
	_, err = service.DiscoverISAPI(context.Background(), ISAPIDiscoverCommand{
		Address: netip.MustParseAddr("127.0.0.1"), MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5},
		Port: port, Username: "admin", PasswordInput: PasswordFromStdin,
	})
	if err == nil || readPassword {
		t.Fatalf("error=%v password read=%v", err, readPassword)
	}
}

func TestISAPIDiscoverRejectsInteractiveMode(t *testing.T) {
	command, err := ParseCLICommand([]string{"isapi-discover", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := DispatchCommand(context.Background(), command, Dependencies{Config: RuntimeUIConfig{Mode: "no"}})
	if err == nil || result.Error == nil || result.Error.Code != FailureUnsupported {
		t.Fatalf("interactive mode was not rejected: %+v, %v", result, err)
	}
}

func TestISAPIDiscoverOneShot(t *testing.T) {
	for _, tt := range []struct {
		name, response string
		challenge      bool
		wantExit       int
		wantCode       string
	}{
		{"verified", `<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress><model>Synthetic</model><serialNumber>TEST</serialNumber></DeviceInfo>`, true, ExitSuccess, ""},
		{"mismatch", `<DeviceInfo><macAddress>00:01:02:03:04:06</macAddress></DeviceInfo>`, true, ExitOperational, "verification"},
		{"malformed", `<DeviceInfo>`, true, ExitOperational, "malformed_response"},
		{"unauthenticated", `<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress></DeviceInfo>`, false, ExitOperational, "authentication"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ISAPI/System/deviceInfo" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if tt.challenge {
					user, password, ok := r.BasicAuth()
					if !ok || user != "admin" || password != "synthetic-password" {
						w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
				}
				_, _ = io.WriteString(w, tt.response)
			}))
			defer server.Close()
			host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := strconv.Atoi(portText); err != nil {
				t.Fatal(err)
			}
			password := oneShotPasswordProvider{reader: io.NopCloser(strings.NewReader("synthetic-password\n"))}
			deps := Dependencies{ISAPIDiscovery: ISAPIKnownAddressService{Passwords: password, Client: server.Client()}}
			args := []string{"--ui=no", "--format=json", "--once", "isapi-discover", "--address", host, "--port", portText, "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin", "--https", "--basic"}
			var out, diagnostics bytes.Buffer
			exit := RunCLIWithTerminalState(context.Background(), args, strings.NewReader(""), &out, &diagnostics, false, false, deps)
			if exit != tt.wantExit {
				t.Fatalf("exit=%d output=%s diagnostics=%s", exit, out.String(), diagnostics.String())
			}
			if strings.Contains(out.String(), "synthetic-password") || strings.Contains(diagnostics.String(), "synthetic-password") {
				t.Fatal("password leaked")
			}
			var result CommandResult
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if tt.wantCode != "" {
				if result.Error == nil || string(result.Error.Code) != tt.wantCode || result.ISAPIDevice != nil {
					t.Fatalf("unexpected failure: %+v", result)
				}
			} else if result.ISAPIDevice == nil || result.ISAPIDevice.Model != "Synthetic" || result.ISAPIDevice.MAC != "00:01:02:03:04:05" || result.ISAPIDevice.Address != host {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}
