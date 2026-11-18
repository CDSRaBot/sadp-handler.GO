package route

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestActivationInvalidInputsStopBeforeGateAndRequests(t *testing.T) {
	var requests, gates atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	target := serverTarget(t, server)
	transport := &ISAPIHTTPTransport{BaseURL: server.URL, Password: "SyntheticPass1!",
		ActivationGate: activationGateFunc(func(context.Context, Target) error { gates.Add(1); return nil }),
		Verifier:       activationVerifierFunc(func(context.Context, Target, string) error { return nil })}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tt := range []struct {
		name     string
		ctx      context.Context
		target   Target
		password string
		wantErr  ErrorKind
	}{
		{name: "canceled", ctx: ctx, target: target, password: "SyntheticPass1!", wantErr: ErrorCanceled},
		{name: "missing MAC", ctx: context.Background(), target: Target{Address: target.Address}, password: "SyntheticPass1!", wantErr: ErrorInvalidInput},
		{name: "unbound interface", ctx: context.Background(), target: Target{Address: target.Address, MAC: target.MAC, Interface: "eth0"}, password: "SyntheticPass1!", wantErr: ErrorInvalidInput},
		{name: "invalid password", ctx: context.Background(), target: target, password: "SyntheticPassword1!", wantErr: ErrorInvalidInput},
	} {
		t.Run(tt.name, func(t *testing.T) {
			transport.Password = tt.password
			result, err := transport.Execute(tt.ctx, OperationActivation, tt.target)
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != tt.wantErr || result.ActivationAttempted ||
				requests.Load() != 0 || gates.Load() != 0 {
				t.Fatalf("result=%+v err=%v requests=%d gates=%d", result, err, requests.Load(), gates.Load())
			}
		})
	}
}
