package route

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestISAPIBasicAuthenticationRequiresOptInAndHTTPS(t *testing.T) {
	tests := []struct {
		name       string
		tls        bool
		allowBasic bool
		success    bool
	}{
		{name: "HTTPS opt in", tls: true, allowBasic: true, success: true},
		{name: "HTTPS default disabled", tls: true},
		{name: "HTTP opt in rejected", allowBasic: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutations := 0
			handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Authorization") == "" {
					writer.Header().Set("WWW-Authenticate", `Basic realm="synthetic"`)
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				username, password, ok := request.BasicAuth()
				if !ok || username != "admin" || password != "synthetic-password" {
					writer.WriteHeader(http.StatusForbidden)
					return
				}
				if request.Method == http.MethodPut {
					mutations++
					_, _ = writer.Write([]byte(`<ResponseStatus><statusCode>1</statusCode></ResponseStatus>`))
					return
				}
				_, _ = writer.Write([]byte(`<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress></DeviceInfo>`))
			})
			var server *httptest.Server
			if tt.tls {
				server = httptest.NewTLSServer(handler)
			} else {
				server = httptest.NewServer(handler)
			}
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Client: server.Client(), Username: "admin", Password: "synthetic-password", AllowBasic: tt.allowBasic}
			target := serverTarget(t, server)
			_, err := transport.discover(context.Background(), target)
			if tt.success && err != nil {
				t.Fatalf("discovery error = %v", err)
			}
			var routeErr *Error
			if !tt.success && (!errors.As(err, &routeErr) || routeErr.Kind != ErrorAuthentication) {
				t.Fatalf("discovery error = %v, want authentication failure", err)
			}
			endpoint, _ := url.Parse(server.URL + "/network/1")
			_, attempted, err := transport.authenticatedMutation(context.Background(), http.MethodPut, endpoint, []byte("<synthetic/>"))
			if tt.success {
				if err != nil || !attempted || mutations != 1 {
					t.Fatalf("mutation error = %v, attempted = %t, count = %d", err, attempted, mutations)
				}
			} else if !errors.As(err, &routeErr) || routeErr.Kind != ErrorAuthentication || attempted || mutations != 0 {
				t.Fatalf("mutation error = %v, attempted = %t, count = %d", err, attempted, mutations)
			}
		})
	}
}

func TestISAPIAuthenticationDoesNotDowngradeFromDigest(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://192.0.2.1/ISAPI/System/deviceInfo", nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := &ISAPIHTTPTransport{Username: "admin", Password: "synthetic-password", AllowBasic: true}
	challenges := []string{`Basic realm="synthetic"`, `Digest realm="synthetic", nonce="test", qop="auth"`}
	header, err := transport.authorizationForChallenge(challenges, request)
	if err != nil || !strings.HasPrefix(header, "Digest ") {
		t.Fatalf("Digest was not preferred: %v", err)
	}
	challenges[1] = `Digest realm="synthetic", nonce="test", qop="unsupported"`
	header, err = transport.authorizationForChallenge(challenges, request)
	if err == nil || header != "" {
		t.Fatal("unsupported Digest challenge downgraded to Basic")
	}
}

func TestISAPIBasicRejectsMalformedChallengeAndUsername(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://192.0.2.1/", nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := &ISAPIHTTPTransport{Username: "admin", Password: "synthetic-password", AllowBasic: true}
	for _, challenge := range []string{"Basic", `Basic charset="UTF-8"`, `Basic realm="one", realm="two"`} {
		header, err := transport.authorizationForChallenge([]string{challenge}, request)
		if err == nil || header != "" {
			t.Fatalf("malformed Basic challenge accepted")
		}
	}
	transport.Username = "admin:other"
	if _, err := transport.authorizationForChallenge([]string{`Basic realm="synthetic"`}, request); err == nil {
		t.Fatal("Basic username containing colon accepted")
	}
	transport.Username = "test\u00e9"
	if _, err := transport.authorizationForChallenge([]string{`Basic realm="synthetic"`}, request); err == nil {
		t.Fatal("non-ASCII Basic username accepted without a UTF-8 challenge")
	}
	if _, err := transport.authorizationForChallenge([]string{`Basic realm="synthetic", charset="UTF-8"`}, request); err != nil {
		t.Fatalf("UTF-8 Basic credentials rejected: %v", err)
	}
	transport.Username = "test\tname"
	if _, err := transport.authorizationForChallenge([]string{`Basic realm="synthetic"`}, request); err == nil {
		t.Fatal("control character in Basic username accepted")
	}
}
