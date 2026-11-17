package route

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestISAPIRouteProbeDoesNotRequireCapabilityEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		t.Error("route probing should not make an HTTP request")
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	provider := ISAPIRouteProvider{Transport: &ISAPIHTTPTransport{BaseURL: server.URL}}
	reachable, err := provider.CanReach(context.Background(), serverTarget(t, server).Address, "")
	if err != nil || !reachable {
		t.Fatalf("reachable = %v, error = %v", reachable, err)
	}
}

func TestISAPIRouteProbeRejectsUnimplementedInterfaceBinding(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	provider := ISAPIRouteProvider{Transport: &ISAPIHTTPTransport{BaseURL: server.URL}}
	reachable, err := provider.CanReach(context.Background(), serverTarget(t, server).Address, "explicit-interface")
	var routeErr *Error
	if reachable || !errors.As(err, &routeErr) || routeErr.Kind != ErrorUnsupported {
		t.Fatalf("reachable = %v, error = %v", reachable, err)
	}
}
