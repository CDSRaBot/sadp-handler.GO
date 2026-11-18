package route

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

type testNetworkProfile struct {
	restart   bool
	buildErr  error
	verifyErr error
}

type testNetworkGate struct{}

func (testNetworkGate) VerifyNetworkDevice(context.Context, Target, DeviceInfo) error {
	return nil
}

func (p testNetworkProfile) BuildUpdate(collection, current []byte, interfaceID string, _ logic.NetworkConfig) ([]byte, error) {
	if string(collection) != "<Interfaces/>" || string(current) != "<Interface/>" || interfaceID != "1" {
		return nil, errors.New("unexpected synthetic profile input")
	}
	if p.buildErr != nil {
		return nil, p.buildErr
	}
	return []byte("<Update/>"), nil
}

func (p testNetworkProfile) RestartRequired([]byte) (bool, error) { return p.restart, nil }
func (p testNetworkProfile) Verify(current []byte, _ logic.NetworkConfig) error {
	if string(current) != "<Interface/>" {
		return errors.New("unexpected synthetic verification input")
	}
	return p.verifyErr
}

type testNetworkLocator struct {
	address netip.Addr
	mac     net.HardwareAddr
	calls   *int
	err     error
}

func (l testNetworkLocator) Await(ctx context.Context, target Target, _ logic.NetworkConfig) (Target, error) {
	*l.calls++
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 60*time.Second {
		return Target{}, errors.New("network workflow deadline is invalid")
	}
	if l.err != nil {
		return Target{}, l.err
	}
	target.Address = l.address
	if l.mac != nil {
		target.MAC = l.mac
	}
	return target, nil
}

func TestISAPINetworkWorkflowUsesVerifiedProfileAndIdentity(t *testing.T) {
	tests := []struct {
		name        string
		profile     testNetworkProfile
		locatorMAC  net.HardwareAddr
		locatorErr  error
		afterMAC    string
		status      string
		wantErr     ErrorKind
		wantSent    bool
		wantCalls   int
		wantSuccess bool
		dhcp        bool
	}{
		{name: "verified", status: "1", wantSent: true, wantCalls: 1, wantSuccess: true},
		{name: "dhcp verified", status: "1", wantSent: true, wantCalls: 1, wantSuccess: true, dhcp: true},
		{name: "unsupported profile", profile: testNetworkProfile{buildErr: errors.New("unsupported")}, status: "1", wantErr: ErrorUnsupported},
		{name: "device rejected", status: "4", wantErr: ErrorRejected, wantSent: true},
		{name: "restart required", profile: testNetworkProfile{restart: true}, status: "1", wantErr: ErrorVerification, wantSent: true},
		{name: "different device", locatorMAC: net.HardwareAddr{0, 1, 2, 3, 4, 6}, status: "1", wantErr: ErrorVerification, wantSent: true, wantCalls: 1},
		{name: "identity changed after mutation", afterMAC: "00:01:02:03:04:06", status: "1", wantErr: ErrorVerification, wantSent: true, wantCalls: 1},
		{name: "locator canceled", locatorErr: context.Canceled, status: "1", wantErr: ErrorCanceled, wantSent: true, wantCalls: 1},
		{name: "state mismatch", profile: testNetworkProfile{verifyErr: errors.New("mismatch")}, status: "1", wantErr: ErrorVerification, wantSent: true, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			putCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Authorization") == "" {
					writer.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				if request.Method == http.MethodPut {
					putCount++
					_, _ = writer.Write([]byte("<ResponseStatus><statusCode>" + tt.status + "</statusCode></ResponseStatus>"))
					return
				}
				switch request.URL.Path {
				case "/ISAPI/System/deviceInfo":
					mac := "00:01:02:03:04:05"
					if putCount > 0 && tt.afterMAC != "" {
						mac = tt.afterMAC
					}
					_, _ = writer.Write([]byte(`<DeviceInfo><macAddress>` + mac + `</macAddress></DeviceInfo>`))
				case "/ISAPI/System/Network/interfaces":
					_, _ = writer.Write([]byte("<Interfaces/>"))
				case "/ISAPI/System/Network/interfaces/1":
					_, _ = writer.Write([]byte("<Interface/>"))
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()
			httpTransport := &http.Transport{
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
				},
			}
			defer httpTransport.CloseIdleConnections()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "synthetic-password", Client: &http.Client{Transport: httpTransport}}
			target := testTarget()
			calls := 0
			plan := logic.NetworkConfig{Address: netip.MustParsePrefix("198.51.100.20/24"), Gateway: netip.MustParseAddr("198.51.100.1")}
			if tt.dhcp {
				plan = logic.NetworkConfig{UseDHCP: true}
			}
			workflow := ISAPINetworkWorkflow{
				Transport: transport,
				Profile:   tt.profile,
				Gate:      testNetworkGate{},
				Locator:   testNetworkLocator{address: netip.MustParseAddr("198.51.100.20"), mac: tt.locatorMAC, calls: &calls, err: tt.locatorErr},
			}
			result, err := workflow.Configure(context.Background(), target, "1", plan)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				var routeErr *Error
				if !errors.As(err, &routeErr) || routeErr.Kind != tt.wantErr {
					t.Fatalf("error = %v, want %s", err, tt.wantErr)
				}
				if tt.locatorErr != nil && !errors.Is(err, tt.locatorErr) {
					t.Fatalf("error = %v does not preserve cancellation", err)
				}
			}
			if result.MutationAttempted != tt.wantSent || result.Verified != tt.wantSuccess || calls != tt.wantCalls ||
				(putCount != 0) != tt.wantSent {
				t.Fatalf("result = %#v, PUTs = %d, locator calls = %d", result, putCount, calls)
			}
		})
	}
}

func TestISAPINetworkWorkflowRejectsInvalidInputsBeforeRequests(t *testing.T) {
	calls := 0
	workflow := ISAPINetworkWorkflow{
		Transport: &ISAPIHTTPTransport{BaseURL: "http://192.0.2.10", Username: "admin", Password: "synthetic-password"},
		Profile:   testNetworkProfile{},
		Gate:      testNetworkGate{},
		Locator:   testNetworkLocator{address: netip.MustParseAddr("198.51.100.20"), calls: &calls},
	}
	plan := logic.NetworkConfig{Address: netip.MustParsePrefix("198.51.100.20/24"), Gateway: netip.MustParseAddr("198.51.100.1")}
	for _, id := range []string{"", ".", "..", "a/b", strings.Repeat("x", 65)} {
		result, err := workflow.Configure(context.Background(), testTarget(), id, plan)
		var routeErr *Error
		if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput || result.MutationAttempted {
			t.Fatalf("interface %q: result = %#v, error = %v", id, result, err)
		}
	}
	plan.Gateway = netip.MustParseAddr("203.0.113.1")
	result, err := workflow.Configure(context.Background(), testTarget(), "1", plan)
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput || result.MutationAttempted || calls != 0 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}
