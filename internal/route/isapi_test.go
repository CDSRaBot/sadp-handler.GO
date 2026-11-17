package route

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func serverTarget(t *testing.T, server *httptest.Server) Target {
	t.Helper()
	base, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	address, err := netip.ParseAddr(base.Hostname())
	if err != nil {
		t.Fatal(err)
	}
	target := testTarget()
	target.Address = address
	return target
}

type activationVerifierFunc func(context.Context, Target, string) error

func (f activationVerifierFunc) VerifyActivation(ctx context.Context, target Target, password string) error {
	return f(ctx, target, password)
}

func TestISAPIHTTPTransportRunsActivationAndVerification(t *testing.T) {
	const challenge = "00112233445566778899aabbccddeeff"
	var activatedPayload activateInfo
	var verified bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/ISAPI/Security/challenge":
			var input securityChallengeRequest
			if err := xml.NewDecoder(request.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}

			modulusBytes, err := base64.StdEncoding.DecodeString(input.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			modulus, err := hex.DecodeString(string(modulusBytes))
			if err != nil {
				t.Fatal(err)
			}
			public := &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: 65537}
			ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, public, []byte(challenge))
			if err != nil {
				t.Fatal(err)
			}
			_, _ = writer.Write([]byte("<Challenge><key>" + base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(ciphertext))) + "</key></Challenge>"))
		case "/ISAPI/System/activate":
			if err := xml.NewDecoder(request.Body).Decode(&activatedPayload); err != nil {
				t.Fatal(err)
			}
			_, _ = writer.Write([]byte("<ResponseStatus><statusCode>1</statusCode></ResponseStatus>"))
		case "/SDK/activateStatus":
			_, _ = writer.Write([]byte("<Status><Activated>true</Activated></Status>"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	transport := &ISAPIHTTPTransport{
		BaseURL: server.URL, Password: "SyntheticPassword1!",
		Verifier: activationVerifierFunc(func(_ context.Context, target Target, password string) error {
			verified = target.Address == serverTarget(t, server).Address && password == "SyntheticPassword1!"
			return nil
		}),
	}
	result, err := transport.Execute(context.Background(), OperationActivation, serverTarget(t, server))
	if err != nil {
		var routeErr *Error
		if errors.As(err, &routeErr) {
			t.Fatalf("activation failed: %s (%v)", routeErr.Kind, routeErr.Err)
		}
		t.Fatal(err)
	}
	if result.Protocol != "isapi" || result.Operation != OperationActivation || !verified {
		t.Fatalf("unexpected result: %#v", result)
	}
	if activatedPayload.Password == "" || strings.Contains(activatedPayload.Password, transport.Password) {
		t.Fatalf("activation payload was not encrypted safely: %q", activatedPayload.Password)
	}
}

func TestISAPIActivationRequiresVerifierBeforeRequests(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	transport := &ISAPIHTTPTransport{BaseURL: server.URL, Password: "SyntheticPassword1!"}
	_, err := transport.Execute(context.Background(), OperationActivation, serverTarget(t, server))
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorUnsupported || requests != 0 {
		t.Fatalf("error = %v, requests = %d", err, requests)
	}
}

func TestISAPIRequestsDoNotFollowRedirects(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		t.Error("redirect destination must not be contacted")
		writer.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusFound)
	}))
	defer server.Close()
	transport := &ISAPIHTTPTransport{BaseURL: server.URL}
	_, err := transport.LookupCapabilities(context.Background(), serverTarget(t, server))
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorRejected {
		t.Fatalf("error = %v, want redirect rejection", err)
	}
}

func TestISAPIRouteAndCapabilityAdaptersSupportHTTPAndHTTPS(t *testing.T) {
	tests := []struct {
		name   string
		server *httptest.Server
		client *http.Client
	}{
		{name: "http", server: httptest.NewServer(nil)},
		{name: "https", server: httptest.NewTLSServer(nil)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.server.Config.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/ISAPI/System/capabilities" {
					http.NotFound(writer, request)
					return
				}
				_, _ = writer.Write([]byte(`<Capabilities><Activation>true</Activation><Network>false</Network><Status>true</Status></Capabilities>`))
			})
			defer test.server.Close()
			if test.name == "https" {
				test.client = test.server.Client()
			}
			transport := &ISAPIHTTPTransport{BaseURL: test.server.URL, Client: test.client}
			target := serverTarget(t, test.server)
			reachable, err := (ISAPIRouteProvider{Transport: transport}).CanReach(context.Background(), target.Address, "")
			if err != nil || !reachable {
				t.Fatalf("CanReach() = %v, %v", reachable, err)
			}
			profile, err := transport.LookupCapabilities(context.Background(), target)
			if err != nil || !profile.Operations[OperationActivation] || profile.Operations[OperationNetwork] {
				t.Fatalf("LookupCapabilities() = %#v, %v", profile, err)
			}
			supported, err := (ISAPICapabilityProvider{Transport: transport}).Supports(context.Background(), target, OperationStatus)
			if err != nil || !supported {
				t.Fatalf("Supports(status) = %v, %v", supported, err)
			}
		})
	}
}

func TestISAPICapabilityLookupHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
			_, _ = writer.Write([]byte(`<Capabilities><Status>true</Status></Capabilities>`))
		}
	}))
	defer server.Close()
	transport := &ISAPIHTTPTransport{BaseURL: server.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := transport.LookupCapabilities(ctx, serverTarget(t, server))
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorTimeout {
		t.Fatalf("expected bounded timeout, got %v", err)
	}
}

func TestISAPIRequestClassifiesAuthenticationAndRejection(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		statusCode int
		want       ErrorKind
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized, want: ErrorAuthentication},
		{name: "forbidden", statusCode: http.StatusForbidden, want: ErrorAuthentication},
		{name: "bad request", statusCode: http.StatusBadRequest, want: ErrorRejected},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.statusCode)
			}))
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL}

			_, err := transport.LookupCapabilities(context.Background(), serverTarget(t, server))
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != testCase.want {
				t.Fatalf("LookupCapabilities() error = %v, want %s", err, testCase.want)
			}
		})
	}
}

func TestISAPIRequestRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("<Capabilities><Status>true</Status></Capabilities>"))
	}))
	defer server.Close()
	transport := &ISAPIHTTPTransport{BaseURL: server.URL, MaxResponseBytes: 8}

	_, err := transport.LookupCapabilities(context.Background(), serverTarget(t, server))
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorMalformed {
		t.Fatalf("LookupCapabilities() error = %v, want malformed response", err)
	}
}

func TestISAPIBaseURLUsesTargetAddress(t *testing.T) {
	transport := &ISAPIHTTPTransport{BaseURL: "https://192.0.2.1:8443"}
	target := Target{Address: netip.MustParseAddr("198.51.100.2")}
	base, err := transport.baseURL(target)
	if err != nil {
		t.Fatal(err)
	}
	if base.String() != "https://198.51.100.2:8443" {
		t.Fatalf("base URL = %q", base.String())
	}
	for _, invalid := range []netip.Addr{{}, netip.IPv4Unspecified(), netip.MustParseAddr("224.0.0.1")} {
		_, err := transport.baseURL(Target{Address: invalid})
		var routeErr *Error
		if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput {
			t.Fatalf("target %s: error = %v", invalid, err)
		}
	}
}
