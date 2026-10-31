package route

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
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
	err   error
}

func (f *fakeRoutes) CanReach(context.Context, netip.Addr, string) (bool, error) {
	f.calls++
	return f.ok, f.err
}

type fakeCapabilities struct {
	calls int
	ok    bool
	err   error
}

func (f *fakeCapabilities) Supports(context.Context, Target, Operation) (bool, error) {
	f.calls++
	return f.ok, f.err
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

func TestSADPUnavailableWithNilISAPITransportReturnsInvalidInput(t *testing.T) {
	sadp := &fakeTransport{err: &Error{Kind: ErrorSADPUnavailable, Err: errors.New("multicast blocked")}}
	selector, err := NewSelector(StrategySADPFirst, Dependencies{
		SADP: sadp, Routes: &fakeRoutes{ok: true}, Capability: &fakeCapabilities{ok: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = selector.Execute(context.Background(), OperationStatus, testTarget())
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput {
		t.Fatalf("expected invalid input, got %v", err)
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
	selector, err := NewSelector(StrategyISAPIOnly, Dependencies{
		SADP: sadp, ISAPI: isapi, Routes: &fakeRoutes{ok: true}, Capability: &fakeCapabilities{ok: true},
	})
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

func TestISAPIOnlyRequiresRouteAndCapabilityBeforeTransport(t *testing.T) {
	isapi := &fakeTransport{}
	selector, err := NewSelector(StrategyISAPIOnly, Dependencies{ISAPI: isapi})
	if err != nil {
		t.Fatal(err)
	}
	_, err = selector.Execute(context.Background(), OperationStatus, testTarget())
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if isapi.calls != 0 {
		t.Fatalf("ISAPI transport called despite missing providers: %d", isapi.calls)
	}
}

func TestISAPIOnlyChecksReachabilityBeforeCapabilityAndTransport(t *testing.T) {
	isapi := &fakeTransport{}
	routes := &fakeRoutes{ok: false}
	capabilities := &fakeCapabilities{ok: true}
	selector, err := NewSelector(StrategyISAPIOnly, Dependencies{
		ISAPI: isapi, Routes: routes, Capability: capabilities,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = selector.Execute(context.Background(), OperationStatus, testTarget())
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorNoRoute {
		t.Fatalf("expected no-route, got %v", err)
	}
	if routes.calls != 1 || capabilities.calls != 0 || isapi.calls != 0 {
		t.Fatalf("unexpected provider calls: routes=%d capabilities=%d isapi=%d", routes.calls, capabilities.calls, isapi.calls)
	}
}

func TestTargetAddressProviderResolvesBeforeGates(t *testing.T) {
	isapi := &fakeTransport{result: Result{Protocol: "isapi"}}
	selector, err := NewSelector(StrategyISAPIOnly, Dependencies{
		ISAPI: isapi, Routes: &fakeRoutes{ok: true}, Capability: &fakeCapabilities{ok: true},
		Addresses: TargetAddressFunc(func(_ context.Context, target Target) (netip.Addr, error) {
			if len(target.MAC) != 6 {
				t.Fatal("address provider received an incomplete target")
			}
			return netip.MustParseAddr("192.0.2.44"), nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := selector.Execute(context.Background(), OperationStatus, Target{MAC: testTarget().MAC})
	if err != nil || result.Target.Address != netip.MustParseAddr("192.0.2.44") {
		t.Fatalf("resolved target = %+v, err=%v", result.Target, err)
	}
}

func TestTargetAddressProviderCancellationIsTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	selector, err := NewSelector(StrategySADPFirst, Dependencies{
		SADP: &fakeTransport{},
		Addresses: TargetAddressFunc(func(ctx context.Context, _ Target) (netip.Addr, error) {
			return netip.Addr{}, ctx.Err()
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = selector.Execute(ctx, OperationStatus, Target{MAC: testTarget().MAC})
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorCanceled {
		t.Fatalf("expected canceled resolution, got %v", err)
	}
}

func TestUnknownOperationRejectedBeforeNetworkIO(t *testing.T) {
	sadp := &fakeTransport{}
	selector, err := NewSelector(StrategySADPFirst, Dependencies{SADP: sadp})
	if err != nil {
		t.Fatal(err)
	}
	_, err = selector.Execute(context.Background(), Operation("bogus"), testTarget())
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if sadp.calls != 0 {
		t.Fatalf("SADP transport called for unknown operation: %d", sadp.calls)
	}
}

func TestProviderContextErrorsPreserveTerminalClassification(t *testing.T) {
	tests := []struct {
		name string
		err  error
		kind ErrorKind
	}{
		{"route canceled", context.Canceled, ErrorCanceled},
		{"capability deadline", context.DeadlineExceeded, ErrorTimeout},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sadp := &fakeTransport{err: &Error{Kind: ErrorSADPUnavailable, Err: errors.New("unavailable")}}
			routes := &fakeRoutes{ok: true}
			capabilities := &fakeCapabilities{ok: true}
			if test.kind == ErrorCanceled {
				routes.err = fmt.Errorf("route provider: %w", test.err)
			} else {
				capabilities.err = fmt.Errorf("capability provider: %w", test.err)
			}
			selector, err := NewSelector(StrategySADPFirst, Dependencies{
				SADP: sadp, ISAPI: &fakeTransport{}, Routes: routes, Capability: capabilities,
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = selector.Execute(context.Background(), OperationStatus, testTarget())
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != test.kind || !errors.Is(err, test.err) {
				t.Fatalf("expected %s preserving cause, got %v", test.kind, err)
			}
		})
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
	selector, err := NewSelector(StrategySADPFirst, Dependencies{SADP: &fakeTransport{}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = selector.Execute(ctx, OperationDiscovery, testTarget())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation cause, got %v", err)
	}
}

func TestSelectorRejectsNilContext(t *testing.T) {
	selector, err := NewSelector(StrategySADPFirst, Dependencies{SADP: &fakeTransport{}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = selector.Execute(nil, OperationDiscovery, testTarget())
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput {
		t.Fatalf("expected invalid-input error, got %v", err)
	}
}

func TestBuildISAPIActivationPasswordUsesSDKSecurityPipeline(t *testing.T) {
	material, err := NewActivationSecurityMaterial("SyntheticPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	challenge := "00112233445566778899aabbccddeeff"
	rsaCiphertext, err := rsa.EncryptPKCS1v15(rand.Reader, &material.PrivateKey.PublicKey, []byte(challenge))
	if err != nil {
		t.Fatal(err)
	}
	challengeValue := base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(rsaCiphertext)))
	encoded, err := BuildISAPIActivationPassword(material, challengeValue)
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
	material, err := NewActivationSecurityMaterial("SyntheticPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildISAPIActivationPassword(
		material,
		"not-a-valid-rsa-challenge",
	)
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorMalformed {
		t.Fatalf("expected malformed challenge error, got %v", err)
	}
}
