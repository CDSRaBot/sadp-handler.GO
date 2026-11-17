package route

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestISAPIMutationAuthenticatesBeforeSendingBody(t *testing.T) {
	putCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			writer.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		putCount++
		body, _ := io.ReadAll(request.Body)
		if request.Method != http.MethodPut || !strings.HasPrefix(request.Header.Get("Authorization"), "Digest ") || string(body) != "<synthetic/>" {
			t.Error("mutation was not authenticated with the expected body")
			writer.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = writer.Write([]byte("<ResponseStatus><statusCode>1</statusCode></ResponseStatus>"))
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL + "/network/1")
	transport := &ISAPIHTTPTransport{Username: "admin", Password: "synthetic-password"}
	_, sent, err := transport.authenticatedMutation(context.Background(), http.MethodPut, endpoint, []byte("<synthetic/>"))
	if err != nil || !sent || putCount != 1 {
		t.Fatalf("error = %v, sent = %t, PUT count = %d", err, sent, putCount)
	}
}

func TestISAPIMutationDoesNotSendWithoutDigestChallenge(t *testing.T) {
	putCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			putCount++
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	transport := &ISAPIHTTPTransport{Username: "admin", Password: "synthetic-password"}
	_, sent, err := transport.authenticatedMutation(context.Background(), http.MethodPut, endpoint, []byte("<synthetic/>"))
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorAuthentication || putCount != 0 || sent {
		t.Fatalf("error = %v, PUT count = %d", err, putCount)
	}
}

func TestISAPIMutationRejectsOversizedBodyBeforeRequests(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	transport := &ISAPIHTTPTransport{Username: "admin", Password: "synthetic-password"}
	_, sent, err := transport.authenticatedMutation(context.Background(), http.MethodPut, endpoint, make([]byte, maxISAPIMutationBody+1))
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput || requests != 0 || sent {
		t.Fatalf("error = %v, requests = %d", err, requests)
	}
}
