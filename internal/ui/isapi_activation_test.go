package ui

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

type isapiActivationGateFunc func(context.Context, route.Target) error

func (f isapiActivationGateFunc) VerifyActivationTarget(ctx context.Context, target route.Target) error {
	return f(ctx, target)
}

func TestISAPIActivationOneShotPipeline(t *testing.T) {
	for _, tt := range []struct {
		name       string
		gate       bool
		deny       bool
		disconnect bool
		wrongMAC   bool
		wantExit   int
		wantCode   string
		attempted  bool
		verified   bool
		password   bool
	}{
		{name: "missing trusted profile", wantExit: ExitOperational, wantCode: "unsupported"},
		{name: "identity gate rejects", gate: true, deny: true, wantExit: ExitOperational, wantCode: "verification"},
		{name: "verified", gate: true, wantExit: ExitSuccess, attempted: true, verified: true, password: true},
		{name: "uncertain delivery", gate: true, disconnect: true, wantExit: ExitOperational, wantCode: "transport", attempted: true, password: true},
		{name: "identity mismatch", gate: true, wrongMAC: true, wantExit: ExitOperational, wantCode: "verification", attempted: true, password: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			var active atomic.Bool
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				switch r.URL.Path {
				case "/SDK/activateStatus":
					if r.Method != http.MethodGet || r.Header.Get("Authorization") != "" {
						t.Error("status request must be unauthenticated GET")
					}
					_, _ = fmt.Fprintf(w, "<Status><Activated>%t</Activated></Status>", active.Load())
				case "/ISAPI/Security/challenge":
					if r.Method != http.MethodPost || r.Header.Get("Authorization") != "" {
						t.Error("challenge must be unauthenticated POST")
					}
					var input struct {
						Key string `xml:"key"`
					}
					if err := xml.NewDecoder(r.Body).Decode(&input); err != nil {
						t.Error(err)
						return
					}
					encoded, err := base64.StdEncoding.DecodeString(input.Key)
					if err != nil {
						t.Error(err)
						return
					}
					modulus, err := hex.DecodeString(string(encoded))
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
				case "/ISAPI/System/activate":
					if r.Method != http.MethodPut || r.Header.Get("Authorization") != "" {
						t.Error("activation must be unauthenticated PUT")
					}
					var payload struct {
						Password string `xml:"password"`
					}
					if err := xml.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Password == "" || strings.Contains(payload.Password, "SyntheticPass1!") {
						t.Error("activation payload must contain an encrypted password")
					}
					if tt.disconnect {
						connection, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = connection.Close()
						return
					}
					active.Store(true)
					_, _ = fmt.Fprint(w, "<ResponseStatus><statusCode>1</statusCode></ResponseStatus>")
				case "/ISAPI/System/deviceInfo":
					user, password, ok := r.BasicAuth()
					if !ok || user != "admin" || password != "SyntheticPass1!" {
						w.Header().Set("WWW-Authenticate", `Basic realm="synthetic"`)
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					mac := "00:01:02:03:04:05"
					if tt.wrongMAC {
						mac = "00:01:02:03:04:06"
					}
					_, _ = fmt.Fprintf(w, "<DeviceInfo><macAddress>%s</macAddress></DeviceInfo>", mac)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			host, port, err := net.SplitHostPort(server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			readPassword := false
			service := ISAPIKnownAddressActivationService{
				Client: server.Client(),
				Passwords: passwordProviderFunc(func(context.Context) (string, error) {
					readPassword = true
					return "SyntheticPass1!", nil
				}),
			}
			if tt.gate {
				service.Gate = isapiActivationGateFunc(func(context.Context, route.Target) error {
					if tt.deny {
						return errors.New("synthetic-secret")
					}
					return nil
				})
			}
			args := []string{"--ui=no", "--format=json", "--once", "isapi-activate", "--address", host,
				"--port", port, "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin", "--https", "--basic"}
			var stdout, stderr bytes.Buffer
			exit := RunCLIWithTerminalState(context.Background(), args, strings.NewReader(""), &stdout, &stderr, false, false,
				Dependencies{ISAPIActivation: service})
			var result CommandResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("invalid output %q: %v", stdout.String(), err)
			}
			if exit != tt.wantExit || result.ISAPIActivation == nil || result.ISAPIActivation.Attempted != tt.attempted ||
				result.ISAPIActivation.Verified != tt.verified || readPassword != tt.password || result.OK != tt.verified {
				t.Fatalf("exit=%d result=%+v password read=%v stderr=%s", exit, result, readPassword, stderr.String())
			}
			if tt.wantCode != "" && (result.Error == nil || string(result.Error.Code) != tt.wantCode) {
				t.Fatalf("unexpected error: %+v", result.Error)
			}
			if strings.Contains(stdout.String()+stderr.String(), "SyntheticPass1!") || strings.Contains(stdout.String()+stderr.String(), "synthetic-secret") {
				t.Fatal("secret leaked to output")
			}
			if (!tt.gate || tt.deny) && requests.Load() != 0 {
				t.Fatalf("requests before gate approval: %d", requests.Load())
			}
		})
	}
}

func TestISAPIActivationRequiresOneShot(t *testing.T) {
	command, err := ParseCLICommand([]string{"isapi-activate", "--address", "192.0.2.1",
		"--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := DispatchCommand(context.Background(), command, Dependencies{})
	if err == nil || result.Error == nil || result.Error.Code != FailureUnsupported {
		t.Fatalf("interactive operation accepted: %+v %v", result, err)
	}
	if _, err := ParseConsoleCommand("isapi-activate --address 192.0.2.1"); err == nil {
		t.Fatal("interactive command accepted")
	}
}
