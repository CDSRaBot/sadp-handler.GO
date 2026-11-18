package route

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
)

func TestActivationStopsBeforeGateWhenRouteUnavailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	target := testTarget()
	target.Address = netip.MustParseAddr("127.0.0.1")
	gateCalled := false
	transport := &ISAPIHTTPTransport{BaseURL: "http://" + address, Password: "SyntheticPass1!",
		ActivationGate: activationGateFunc(func(context.Context, Target) error { gateCalled = true; return nil }),
		Verifier:       activationVerifierFunc(func(context.Context, Target, string) error { return nil })}
	result, err := transport.Execute(context.Background(), OperationActivation, target)
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorNoRoute || result.ActivationAttempted || gateCalled {
		t.Fatalf("result=%+v error=%v gate=%v", result, err, gateCalled)
	}
}
