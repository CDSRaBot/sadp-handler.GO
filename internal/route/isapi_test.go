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
	"strings"
	"testing"
)

func TestISAPIHTTPTransportRunsActivationAndVerification(t *testing.T) {
	const challenge = "00112233445566778899aabbccddeeff"
	var activatedPayload activateInfo
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
			_, _ = writer.Write([]byte("<Response><statusCode>1</statusCode></Response>"))
		case "/SDK/activateStatus":
			_, _ = writer.Write([]byte("<Status><Activated>true</Activated></Status>"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	transport := &ISAPIHTTPTransport{BaseURL: server.URL, Password: "SyntheticPassword1!"}
	result, err := transport.Execute(context.Background(), OperationActivation, testTarget())
	if err != nil {
		var routeErr *Error
		if errors.As(err, &routeErr) {
			t.Fatalf("activation failed: %s (%v)", routeErr.Kind, routeErr.Err)
		}
		t.Fatal(err)
	}
	if result.Protocol != "isapi" || result.Operation != OperationActivation {
		t.Fatalf("unexpected result: %#v", result)
	}
	if activatedPayload.Password == "" || strings.Contains(activatedPayload.Password, transport.Password) {
		t.Fatalf("activation payload was not encrypted safely: %q", activatedPayload.Password)
	}
}
