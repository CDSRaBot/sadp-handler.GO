package route

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActivationChallengeRequiresUnambiguousDirectKey(t *testing.T) {
	tests := []struct {
		payload string
		valid   bool
	}{
		{`<Challenge><key>synthetic</key></Challenge>`, true},
		{`<Challenge xmlns="urn:example"><key>synthetic</key></Challenge>`, true},
		{`<Other><key>synthetic</key></Other>`, false},
		{`<Challenge><key>one</key><key>two</key></Challenge>`, false},
		{`<Challenge><key>one</key><other><key>two</key></other></Challenge>`, false},
		{`<Challenge><key><value>one</value></key></Challenge>`, false},
		{`<Challenge><key>one</key></Challenge><Challenge/>`, false},
	}
	for _, tt := range tests {
		_, err := parseUniqueDirectText([]byte(tt.payload), "Challenge", "key")
		if (err == nil) != tt.valid {
			t.Errorf("payload %q: error = %v", tt.payload, err)
		}
	}
}

func TestActivationDoesNotMutateAfterAmbiguousChallenge(t *testing.T) {
	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/ISAPI/Security/challenge" {
			_, _ = writer.Write([]byte(`<Challenge><key>one</key><key>two</key></Challenge>`))
			return
		}
		mutations++
	}))
	defer server.Close()
	transport := &ISAPIHTTPTransport{
		BaseURL: server.URL, Password: "SyntheticPass1!",
		Verifier: activationVerifierFunc(func(context.Context, Target, string) error { return nil }),
	}
	_, err := transport.Execute(context.Background(), OperationActivation, serverTarget(t, server))
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorMalformed || mutations != 0 {
		t.Fatalf("error = %v, mutations = %d", err, mutations)
	}
}

func TestActivationRejectsPasswordThatWouldBeTruncatedBeforeRequests(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()
	transport := &ISAPIHTTPTransport{
		BaseURL: server.URL, Password: "SyntheticPassword1!",
		Verifier: activationVerifierFunc(func(context.Context, Target, string) error { return nil }),
	}
	_, err := transport.Execute(context.Background(), OperationActivation, serverTarget(t, server))
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput || requests != 0 {
		t.Fatalf("error = %v, requests = %d", err, requests)
	}
}

func TestActivationPasswordLengthBoundaries(t *testing.T) {
	material, err := NewActivationSecurityMaterial("A123456789012345")
	if err != nil {
		t.Fatalf("16-character password rejected: %v", err)
	}
	material.Password = "A1234567890123456"
	_, err = BuildISAPIActivationPassword(material, "invalid-challenge")
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput {
		t.Fatalf("17-character password error = %v", err)
	}
}

func TestActivationStatusRejectsAmbiguousResponses(t *testing.T) {
	tests := []struct {
		payload string
		want    ErrorKind
	}{
		{`<Status><Activated>true</Activated></Status>`, ""},
		{`<Status><Activated> 1 </Activated></Status>`, ""},
		{`<Status><Activated>false</Activated></Status>`, ErrorVerification},
		{`<Status><Activated>unknown</Activated></Status>`, ErrorMalformed},
		{`<Status/>`, ErrorMalformed},
		{`<Status><Activated>false</Activated><Activated>true</Activated></Status>`, ErrorMalformed},
		{`<Status><Activated>true</Activated><other><Activated>false</Activated></other></Status>`, ErrorMalformed},
		{`<Status><Activated>true</Activated></Status><Status/>`, ErrorMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(tt.payload))
			}))
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL}
			_, err := transport.Execute(context.Background(), OperationStatus, serverTarget(t, server))
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != tt.want {
				t.Fatalf("error = %v, want %s", err, tt.want)
			}
		})
	}
}
