package route

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestISAPIDeviceInfoVerifierRequiresAuthenticatedMatchingIdentity(t *testing.T) {
	tests := []struct {
		name      string
		challenge bool
		response  string
		wantErr   ErrorKind
	}{
		{name: "verified", challenge: true, response: `<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress></DeviceInfo>`},
		{name: "wrong identity", challenge: true, response: `<DeviceInfo><macAddress>00:01:02:03:04:06</macAddress></DeviceInfo>`, wantErr: ErrorVerification},
		{name: "missing identity", challenge: true, response: `<DeviceInfo/>`, wantErr: ErrorVerification},
		{name: "malformed", challenge: true, response: `<DeviceInfo>`, wantErr: ErrorMalformed},
		{name: "unauthenticated", response: `<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress></DeviceInfo>`, wantErr: ErrorAuthentication},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if tt.challenge && request.Header.Get("Authorization") == "" {
					writer.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = writer.Write([]byte(tt.response))
			}))
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin"}
			verifier := ISAPIDeviceInfoVerifier{Transport: transport}
			err := verifier.VerifyActivation(context.Background(), serverTarget(t, server), "synthetic-password")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("verification failed: %v", err)
				}
				return
			}
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != tt.wantErr {
				t.Fatalf("error = %v, want %s", err, tt.wantErr)
			}
		})
	}
}
