package route

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
)

type fakeTransport struct {
	calls  int
	result Result
	err    error
}

func (f *fakeTransport) Execute(context.Context, Operation, Target) (Result, error) {
	f.calls++
	return f.result, f.err
}

type fakeRoutes struct {
	calls int
	ok    bool
}

func (f *fakeRoutes) CanReach(context.Context, netip.Addr, string) (bool, error) {
	f.calls++
	return f.ok, nil
}

type fakeCapabilities struct {
	calls int
	ok    bool
}

func (f *fakeCapabilities) Supports(context.Context, Target, Operation) (bool, error) {
	f.calls++
	return f.ok, nil
}

func testTarget() Target {
	return Target{
		MAC:     net.HardwareAddr{0, 1, 2, 3, 4, 5},
		Address: netip.MustParseAddr("192.0.2.10"),
	}
}

func TestNewSelectorUsesSADPFirstByDefault(t *testing.T) {
	sadp := &fakeTransport{result: Result{Protocol: "sadp"}}
	_, err := NewSelector("", Dependencies{SADP: sadp})
	if err != nil {
		t.Fatalf("construct selector: %v", err)
	}
	if sadp.calls != 0 {
		t.Fatalf("constructor performed network work: %d calls", sadp.calls)
	}
}

func TestSADPSuccessPreventsFallback(t *testing.T) {
	sadp := &fakeTransport{result: Result{Protocol: "sadp"}}
	isapi := &fakeTransport{result: Result{Protocol: "isapi"}}
	selector, err := NewSelector(StrategySADPFirst, Dependencies{SADP: sadp, ISAPI: isapi})
	if err != nil {
		t.Fatal(err)
	}
	result, err := selector.Execute(context.Background(), OperationActivation, testTarget())
	if err != nil {
		t.Fatal(err)
	}
	if result.Protocol != "sadp" || sadp.calls != 1 || isapi.calls != 0 {
		t.Fatalf("unexpected route: result=%+v sadp=%d isapi=%d", result, sadp.calls, isapi.calls)
	}
}

func TestSADPUnavailableFallsBackOnlyWithRouteAndCapability(t *testing.T) {
	sadp := &fakeTransport{err: &Error{Kind: ErrorSADPUnavailable, Err: errors.New("multicast blocked")}}
	isapi := &fakeTransport{result: Result{Protocol: "isapi", Fallback: FallbackEligible}}
	routes := &fakeRoutes{ok: true}
	capabilities := &fakeCapabilities{ok: true}
	selector, err := NewSelector(StrategySADPFirst, Dependencies{
		SADP: sadp, ISAPI: isapi, Routes: routes, Capability: capabilities,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := selector.Execute(context.Background(), OperationNetwork, testTarget())
	if err != nil {
		t.Fatal(err)
	}
	if result.Protocol != "isapi" || routes.calls != 1 || capabilities.calls != 1 {
		t.Fatalf("fallback was not gated: result=%+v routes=%d capabilities=%d", result, routes.calls, capabilities.calls)
	}
}

func TestUnavailableRouteDoesNotFallbackWithoutReachability(t *testing.T) {
	sadp := &fakeTransport{err: &Error{Kind: ErrorSADPUnavailable, Err: errors.New("timeout")}}
	isapi := &fakeTransport{}
	routes := &fakeRoutes{ok: false}
	capabilities := &fakeCapabilities{ok: true}
	selector, err := NewSelector(StrategySADPFirst, Dependencies{
		SADP: sadp, ISAPI: isapi, Routes: routes, Capability: capabilities,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = selector.Execute(context.Background(), OperationStatus, testTarget())
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorNoRoute {
		t.Fatalf("expected no-route error, got %v", err)
	}
	if isapi.calls != 0 || capabilities.calls != 0 {
		t.Fatalf("unreachable target was sent to ISAPI: isapi=%d capabilities=%d", isapi.calls, capabilities.calls)
	}
}

func TestRejectionNeverFallsBack(t *testing.T) {
	sadp := &fakeTransport{err: &Error{Kind: ErrorRejected, Err: errors.New("rejected")}}
	isapi := &fakeTransport{}
	selector, err := NewSelector(StrategySADPFirst, Dependencies{SADP: sadp, ISAPI: isapi})
	if err != nil {
		t.Fatal(err)
	}
	_, err = selector.Execute(context.Background(), OperationActivation, testTarget())
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorRejected {
		t.Fatalf("expected rejection, got %v", err)
	}
	if isapi.calls != 0 {
		t.Fatalf("rejection triggered fallback: %d calls", isapi.calls)
	}
}

func TestAuthenticationFailureNeverFallsBack(t *testing.T) {
	sadp := &fakeTransport{err: &Error{Kind: ErrorAuthentication, Err: errors.New("invalid credentials")}}
	isapi := &fakeTransport{}
	selector, err := NewSelector(StrategySADPFirst, Dependencies{SADP: sadp, ISAPI: isapi})
	if err != nil {
		t.Fatal(err)
	}
	_, err = selector.Execute(context.Background(), OperationActivation, testTarget())
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorAuthentication {
		t.Fatalf("expected authentication error, got %v", err)
	}
	if isapi.calls != 0 {
		t.Fatalf("authentication failure triggered fallback: %d calls", isapi.calls)
	}
}

func TestExplicitISAPIDoesNotAttemptSADP(t *testing.T) {
	sadp := &fakeTransport{}
	isapi := &fakeTransport{result: Result{Protocol: "isapi"}}
	selector, err := NewSelector(StrategyISAPIOnly, Dependencies{SADP: sadp, ISAPI: isapi})
	if err != nil {
		t.Fatal(err)
	}
	result, err := selector.Execute(context.Background(), OperationStatus, testTarget())
	if err != nil {
		t.Fatal(err)
	}
	if result.Protocol != "isapi" || sadp.calls != 0 || isapi.calls != 1 {
		t.Fatalf("unexpected explicit route: result=%+v sadp=%d isapi=%d", result, sadp.calls, isapi.calls)
	}
}

func TestRouteErrorsRedactSecurityMaterial(t *testing.T) {
	secret := "password challenge nonce rsa aes Authorization XML payload"
	err := &Error{Kind: ErrorMalformed, Err: errors.New(secret)}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("route error leaked security material: %q", err.Error())
	}
}

func TestSelectorPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	selector, err := NewSelector(StrategySADPFirst, Dependencies{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = selector.Execute(ctx, OperationDiscovery, testTarget())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation cause, got %v", err)
	}
}

func TestBuildISAPIActivationPasswordUsesSDKSecurityPipeline(t *testing.T) {
	material := ActivationSecurityMaterial{
		PublicKey: "synthetic-base64-ascii-hex-rsa-modulus",
		Password:  "SyntheticPassword1!",
	}
	encoded, err := BuildISAPIActivationPassword(material, "synthetic-challenge-ciphertext")
	if err != nil {
		t.Fatal(err)
	}
	if encoded == "" {
		t.Fatal("expected Base64 ASCII-hex AES ciphertext")
	}
	if strings.Contains(encoded, material.Password) {
		t.Fatalf("activation payload leaked plaintext password: %q", encoded)
	}
}

func TestBuildISAPIActivationPasswordRejectsMalformedChallenge(t *testing.T) {
	_, err := BuildISAPIActivationPassword(
		ActivationSecurityMaterial{PublicKey: "synthetic", Password: "SyntheticPassword1!"},
		"not-a-valid-rsa-challenge",
	)
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorMalformed {
		t.Fatalf("expected malformed challenge error, got %v", err)
	}
}
