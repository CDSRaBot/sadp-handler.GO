package route

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestISAPIDiscoveryByKnownAddressPreservesIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/ISAPI/System/deviceInfo" || request.Method != http.MethodGet {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if request.Header.Get("Authorization") == "" {
			writer.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress><model>TEST</model><serialNumber>SYNTHETIC</serialNumber><firmwareVersion>1.0</firmwareVersion></DeviceInfo>`))
	}))
	defer server.Close()
	transport := &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "synthetic-password"}
	target := serverTarget(t, server)
	target.MAC = nil
	selector, err := NewSelector(StrategyISAPIOnly, Dependencies{
		ISAPI: transport, Routes: ISAPIRouteProvider{Transport: transport},
		Capability: ISAPICapabilityProvider{Transport: transport},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := selector.Execute(context.Background(), OperationDiscovery, target)
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.MAC.String() != "00:01:02:03:04:05" || result.Target.Address != target.Address ||
		result.Device == nil || result.Device.Model != "TEST" ||
		result.Device.SerialNumber != "SYNTHETIC" || result.Device.FirmwareVersion != "1.0" {
		t.Fatalf("unexpected discovery result: %#v", result)
	}
}

func TestISAPIDiscoveryRejectsMismatchedMAC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") == "" {
			writer.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`<DeviceInfo><macAddress>00:01:02:03:04:06</macAddress></DeviceInfo>`))
	}))
	defer server.Close()
	transport := &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "synthetic-password"}
	result, err := transport.Execute(context.Background(), OperationDiscovery, serverTarget(t, server))
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorVerification || result.Device != nil {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}
