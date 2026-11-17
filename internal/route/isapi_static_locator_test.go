package route

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

func TestISAPIStaticLocatorWaitsForAuthenticatedIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") == "" {
			writer.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress></DeviceInfo>`))
	}))
	defer server.Close()
	attempts := 0
	httpTransport := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("synthetic device not ready")
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}}
	defer httpTransport.CloseIdleConnections()
	transport := &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "synthetic-password", Client: &http.Client{Transport: httpTransport}}
	locator := ISAPIStaticNetworkLocator{Transport: transport, RetryInterval: time.Millisecond}
	plan := logic.NetworkConfig{Address: netip.MustParsePrefix("198.51.100.20/24"), Gateway: netip.MustParseAddr("198.51.100.1")}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	target, err := locator.Await(ctx, testTarget(), plan)
	if err != nil || target.Address != plan.Address.Addr() || attempts < 2 {
		t.Fatalf("target = %#v, attempts = %d, error = %v", target, attempts, err)
	}
}

func TestISAPIStaticLocatorRejectsWrongIdentityAndDHCP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") == "" {
			writer.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`<DeviceInfo><macAddress>00:01:02:03:04:06</macAddress></DeviceInfo>`))
	}))
	defer server.Close()
	httpTransport := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}}
	defer httpTransport.CloseIdleConnections()
	locator := ISAPIStaticNetworkLocator{
		Transport:     &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "synthetic-password", Client: &http.Client{Transport: httpTransport}},
		RetryInterval: time.Millisecond,
	}
	plan := logic.NetworkConfig{Address: netip.MustParsePrefix("198.51.100.20/24"), Gateway: netip.MustParseAddr("198.51.100.1")}
	_, err := locator.Await(context.Background(), testTarget(), plan)
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorVerification {
		t.Fatalf("wrong identity error = %v", err)
	}
	_, err = locator.Await(context.Background(), testTarget(), logic.NetworkConfig{UseDHCP: true})
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput {
		t.Fatalf("DHCP error = %v", err)
	}
}

func TestISAPIStaticLocatorHonorsCancellation(t *testing.T) {
	locator := ISAPIStaticNetworkLocator{
		Transport: &ISAPIHTTPTransport{BaseURL: "http://192.0.2.1", Username: "admin", Password: "synthetic-password"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	plan := logic.NetworkConfig{Address: netip.MustParsePrefix("198.51.100.20/24"), Gateway: netip.MustParseAddr("198.51.100.1")}
	_, err := locator.Await(ctx, testTarget(), plan)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
}
