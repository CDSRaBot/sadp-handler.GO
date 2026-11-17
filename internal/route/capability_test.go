package route

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestISAPICapabilitiesRejectAmbiguousDocuments(t *testing.T) {
	for _, payload := range []string{
		`<Other><Activation>true</Activation></Other>`,
		`<Capabilities><Activation><Value>true</Value></Activation></Capabilities>`,
		`<Capabilities><Activation>true</Activation></Capabilities><Capabilities/>`,
		`<Capabilities><Activation>true</Activation><Activation>false</Activation></Capabilities>`,
	} {
		t.Run(payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(payload))
			}))
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL}
			_, err := transport.LookupCapabilities(context.Background(), serverTarget(t, server))
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != ErrorMalformed {
				t.Fatalf("error = %v, want malformed response", err)
			}
		})
	}
}
