package route

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDigestAuthenticationReplaysRequestOnce(t *testing.T) {
	for _, algorithm := range []string{"MD5", "SHA-256"} {
		t.Run(algorithm, func(t *testing.T) {
			const body = "<Synthetic>content</Synthetic>"
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls++
				input, err := io.ReadAll(request.Body)
				if err != nil || string(input) != body {
					t.Errorf("request body = %q, err = %v", input, err)
				}
				header := request.Header.Get("Authorization")
				if header == "" {
					writer.Header().Set("WWW-Authenticate", `Digest realm="test realm", nonce="test nonce", qop="auth,auth-int", algorithm=`+algorithm+`, opaque="test opaque"`)
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				if !strings.HasPrefix(header, "Digest ") {
					t.Errorf("unexpected authentication scheme")
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				params, err := parseDigestParameters(strings.TrimPrefix(header, "Digest "))
				if err != nil {
					t.Errorf("invalid authorization header: %v", err)
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				hash := func(value string) string {
					if algorithm == "SHA-256" {
						sum := sha256.Sum256([]byte(value))
						return hex.EncodeToString(sum[:])
					}
					sum := md5.Sum([]byte(value))
					return hex.EncodeToString(sum[:])
				}
				ha1 := hash("admin:test realm:synthetic-password")
				ha2 := hash("PUT:" + request.URL.RequestURI())
				want := hash(ha1 + ":test nonce:00000001:" + params["cnonce"] + ":auth:" + ha2)
				if params["response"] != want || params["uri"] != request.URL.RequestURI() ||
					params["opaque"] != "test opaque" || params["nc"] != "00000001" {
					t.Error("Digest response does not match request")
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = writer.Write([]byte("ok"))
			}))
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "synthetic-password"}
			endpoint, _ := url.Parse(server.URL + "/device?mode=1")
			result, err := transport.request(context.Background(), http.MethodPut, endpoint, []byte(body))
			if err != nil || string(result) != "ok" || calls != 2 {
				t.Fatalf("result = %q, err = %v, calls = %d", result, err, calls)
			}
		})
	}
}

func TestDigestAuthenticationRejectsInvalidChallengeAndRepeated401(t *testing.T) {
	for _, challenge := range []string{
		`Basic realm="test"`,
		`Digest realm="test", nonce="test", qop="auth", algorithm=unsupported`,
		`Digest realm="test", nonce="test", nonce="duplicate", qop="auth"`,
		`Digest realm="test", nonce="test", qop="auth-int"`,
		`Digest realm="test", nonce="test", qop="auth"`,
	} {
		t.Run(challenge, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				calls++
				writer.Header().Set("WWW-Authenticate", challenge)
				writer.WriteHeader(http.StatusUnauthorized)
			}))
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Username: "admin", Password: "synthetic-password"}
			endpoint, _ := url.Parse(server.URL)
			_, err := transport.request(context.Background(), http.MethodGet, endpoint, nil)
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != ErrorAuthentication || calls > 2 {
				t.Fatalf("err = %v, calls = %d", err, calls)
			}
			if strings.Contains(err.Error(), "synthetic-password") {
				t.Fatal("authentication error leaked password")
			}
		})
	}
}
