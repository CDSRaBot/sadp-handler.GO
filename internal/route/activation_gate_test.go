package route

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func writeSyntheticActivationChallenge(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var input securityChallengeRequest
	if err := xml.NewDecoder(r.Body).Decode(&input); err != nil {
		t.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	encodedModulus, err := base64.StdEncoding.DecodeString(input.PublicKey)
	if err != nil {
		t.Error(err)
		return
	}
	modulus, err := hex.DecodeString(string(encodedModulus))
	if err != nil {
		t.Error(err)
		return
	}
	key := &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: 65537}
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, key, []byte("00112233445566778899aabbccddeeff"))
	if err != nil {
		t.Error(err)
		return
	}
	_, _ = fmt.Fprintf(w, "<Challenge><key>%s</key></Challenge>", base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(ciphertext))))
}

func TestActivationRequiresTrustedTargetAndInactiveState(t *testing.T) {
	tests := []struct {
		name       string
		gate       ISAPIActivationGate
		status     string
		wantErr    ErrorKind
		wantStatus int32
		wantPost   int32
	}{
		{name: "missing gate", wantErr: ErrorUnsupported},
		{name: "untrusted target", gate: activationGateFunc(func(context.Context, Target) error {
			return &Error{Kind: ErrorVerification, Err: errors.New("unverified target")}
		}), wantErr: ErrorVerification},
		{name: "gate changes MAC", gate: activationGateFunc(func(_ context.Context, target Target) error {
			target.MAC[0] = 0xff
			return nil
		}), wantErr: ErrorVerification},
		{name: "already active", gate: activationGateFunc(func(context.Context, Target) error { return nil }), status: "true", wantErr: ErrorRejected, wantStatus: 1},
		{name: "malformed state", gate: activationGateFunc(func(context.Context, Target) error { return nil }), status: "unknown", wantErr: ErrorMalformed, wantStatus: 1},
		{name: "inactive, malformed challenge", gate: activationGateFunc(func(context.Context, Target) error { return nil }), status: "false", wantErr: ErrorMalformed, wantStatus: 1, wantPost: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var statuses, challenges, mutations atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/SDK/activateStatus":
					statuses.Add(1)
					_, _ = fmt.Fprintf(w, "<Status><Activated>%s</Activated></Status>", tt.status)
				case "/ISAPI/Security/challenge":
					challenges.Add(1)
					_, _ = w.Write([]byte("<Challenge/>"))
				default:
					mutations.Add(1)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Password: "SyntheticPass1!", ActivationGate: tt.gate,
				Verifier: activationVerifierFunc(func(context.Context, Target, string) error { return nil })}
			target := serverTarget(t, server)
			result, err := transport.Execute(context.Background(), OperationActivation, target)
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != tt.wantErr || result.ActivationAttempted ||
				statuses.Load() != tt.wantStatus || challenges.Load() != tt.wantPost || mutations.Load() != 0 ||
				target.MAC[0] != 0 || result.Target.MAC[0] != 0 {
				t.Fatalf("result=%+v error=%v status=%d challenge=%d mutations=%d",
					result, err, statuses.Load(), challenges.Load(), mutations.Load())
			}
		})
	}
}

func TestActivationAttemptedSurvivesUncertainDeliveryAndVerificationFailure(t *testing.T) {
	for _, tt := range []struct {
		name       string
		disconnect bool
		want       ErrorKind
	}{
		{"connection lost during PUT", true, ErrorTransport},
		{"verification failed", false, ErrorVerification},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var activated atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/SDK/activateStatus":
					_, _ = fmt.Fprintf(w, "<Status><Activated>%t</Activated></Status>", activated.Load())
				case "/ISAPI/Security/challenge":
					writeSyntheticActivationChallenge(t, w, r)
				case "/ISAPI/System/activate":
					if tt.disconnect {
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = conn.Close()
						return
					}
					activated.Store(true)
					_, _ = w.Write([]byte("<ResponseStatus><statusCode>1</statusCode></ResponseStatus>"))
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Password: "SyntheticPass1!",
				ActivationGate: activationGateFunc(func(context.Context, Target) error { return nil }),
				Verifier:       activationVerifierFunc(func(context.Context, Target, string) error { return errors.New("verification failed") })}
			result, err := transport.Execute(context.Background(), OperationActivation, serverTarget(t, server))
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != tt.want || !result.ActivationAttempted {
				t.Fatalf("result=%+v error=%v, want attempted and %s", result, err, tt.want)
			}
		})
	}
}

func TestSelectorPreservesActivationAttemptedOnError(t *testing.T) {
	failure := &Error{Kind: ErrorTransport, Err: errors.New("synthetic transport failure")}
	transport := &fakeTransport{result: Result{ActivationAttempted: true}, err: failure}
	selector, err := NewSelector(StrategyISAPIOnly, Dependencies{
		ISAPI: transport, Routes: &fakeRoutes{ok: true}, Capability: &fakeCapabilities{ok: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := selector.Execute(context.Background(), OperationActivation, testTarget())
	if !errors.Is(err, failure) || !result.ActivationAttempted || result.Err != failure {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestActivationDoesNotRetryWithNewCredentials(t *testing.T) {
	for _, tt := range []struct {
		name        string
		rejectPath  string
		wantAttempt bool
	}{
		{name: "challenge", rejectPath: "/ISAPI/Security/challenge"},
		{name: "activation", rejectPath: "/ISAPI/System/activate", wantAttempt: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var rejected, authenticated atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					authenticated.Add(1)
				}
				if r.URL.Path == tt.rejectPath {
					rejected.Add(1)
					w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				switch r.URL.Path {
				case "/SDK/activateStatus":
					_, _ = w.Write([]byte("<Status><Activated>false</Activated></Status>"))
				case "/ISAPI/Security/challenge":
					writeSyntheticActivationChallenge(t, w, r)
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			serverURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			jar.SetCookies(serverURL, []*http.Cookie{{Name: "session", Value: "synthetic"}})
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "SyntheticPass1!",
				Client:         &http.Client{Jar: jar},
				ActivationGate: activationGateFunc(func(context.Context, Target) error { return nil }),
				Verifier:       activationVerifierFunc(func(context.Context, Target, string) error { return nil })}
			result, err := transport.Execute(context.Background(), OperationActivation, serverTarget(t, server))
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != ErrorAuthentication || rejected.Load() != 1 ||
				authenticated.Load() != 0 || result.ActivationAttempted != tt.wantAttempt {
				t.Fatalf("result=%+v err=%v rejected=%d authenticated=%d",
					result, err, rejected.Load(), authenticated.Load())
			}
		})
	}
}
