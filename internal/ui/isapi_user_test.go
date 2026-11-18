package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

type syntheticUserUIProfile struct{ deny bool }

func (p syntheticUserUIProfile) VerifyUserDevice(_ context.Context, _ route.Target, info route.DeviceInfo, _ route.ISAPIUserOperation) error {
	if p.deny || info.Model != "Synthetic" {
		return errors.New("synthetic-secret")
	}
	return nil
}

func (syntheticUserUIProfile) EncryptionKey(context.Context, *route.ISAPIHTTPTransport, route.Target, route.ISAPIUserOperation) (route.ISAPIUserSecurity, []byte, error) {
	return route.ISAPIUserSecurityAES128, bytes.Repeat([]byte{7}, 16), nil
}

func (syntheticUserUIProfile) BuildUserMutation(request route.ISAPIUserRequest, encrypted route.ISAPIEncryptedField) (string, []byte, error) {
	if request.Password != "" {
		return "", nil, errors.New("plaintext password passed to schema")
	}
	body, err := xml.Marshal(struct {
		XMLName  xml.Name `xml:"User"`
		Password string   `xml:"password"`
	}{Password: encrypted.Ciphertext})
	if request.Operation == route.ISAPIUserCreate {
		return http.MethodPost, body, err
	}
	return http.MethodPut, body, err
}

func (syntheticUserUIProfile) VerifyUserState(request route.ISAPIUserRequest, state []byte) error {
	if request.Password != "" || string(state) != "<User><id>2</id><userName>synthetic</userName></User>" {
		return errors.New("synthetic-secret")
	}
	return nil
}

func TestISAPIUserOneShotPipeline(t *testing.T) {
	for _, tt := range []struct {
		name       string
		profile    bool
		deny       bool
		create     bool
		disconnect bool
		wantExit   int
		wantCode   string
		attempted  bool
		verified   bool
		readInput  bool
	}{
		{name: "missing profile", wantExit: ExitOperational, wantCode: "unsupported"},
		{name: "profile rejected", profile: true, deny: true, wantExit: ExitOperational, wantCode: "unsupported", readInput: true},
		{name: "create verified", profile: true, create: true, wantExit: ExitSuccess, attempted: true, verified: true, readInput: true},
		{name: "update verified", profile: true, wantExit: ExitSuccess, attempted: true, verified: true, readInput: true},
		{name: "uncertain delivery", profile: true, disconnect: true, wantExit: ExitOperational, wantCode: "transport", attempted: true, readInput: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requests, writes atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				user, password, ok := r.BasicAuth()
				if !ok || user != "admin" || password != "synthetic-admin" {
					w.Header().Set("WWW-Authenticate", `Basic realm="synthetic"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.Method == http.MethodPut || r.Method == http.MethodPost {
					writes.Add(1)
					body, err := io.ReadAll(r.Body)
					if err != nil || bytes.Contains(body, []byte("SyntheticUser1!")) || !bytes.Contains(body, []byte("<password>")) {
						t.Error("unsafe user mutation body")
					}
					if tt.disconnect {
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = conn.Close()
						return
					}
					_, _ = fmt.Fprint(w, "<ResponseStatus><statusCode>1</statusCode></ResponseStatus>")
					return
				}
				switch r.URL.Path {
				case "/ISAPI/System/deviceInfo":
					_, _ = fmt.Fprint(w, "<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress><model>Synthetic</model></DeviceInfo>")
				case "/ISAPI/Security/users/2":
					_, _ = fmt.Fprint(w, "<User><id>2</id><userName>synthetic</userName></User>")
				default:
					t.Errorf("unexpected authenticated request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			host, port, err := net.SplitHostPort(server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			readInput := false
			input := oneShotPasswordProvider{reader: io.NopCloser(strings.NewReader(`{"admin_password":"synthetic-admin","user_password":"SyntheticUser1!"}` + "\n"))}
			service := ISAPIKnownAddressUserService{
				Client: server.Client(),
				Passwords: passwordProviderFunc(func(ctx context.Context) (string, error) {
					readInput = true
					return input.Password(ctx)
				}),
			}
			if tt.profile {
				service.Profile = syntheticUserUIProfile{deny: tt.deny}
			}
			args := []string{"--ui=no", "--format=json", "--once", "isapi-user", "--address", host,
				"--port", port, "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin",
				"--https", "--basic", "--user-id", "2", "--user-name", "synthetic", "--yes"}
			if tt.create {
				args = append(args, "--create")
			} else {
				args = append(args, "--update")
			}
			var stdout, stderr bytes.Buffer
			exit := RunCLIWithTerminalState(context.Background(), args, strings.NewReader(""), &stdout, &stderr, false, false,
				Dependencies{ISAPIUser: service})
			var result CommandResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("invalid output %q: %v", stdout.String(), err)
			}
			if exit != tt.wantExit || result.ISAPIUser == nil || result.ISAPIUser.Attempted != tt.attempted ||
				result.ISAPIUser.Verified != tt.verified || readInput != tt.readInput || result.OK != tt.verified ||
				(writes.Load() != 0) != tt.attempted {
				t.Fatalf("exit=%d result=%+v input=%v writes=%d stderr=%s", exit, result, readInput, writes.Load(), stderr.String())
			}
			if tt.wantCode != "" && (result.Error == nil || string(result.Error.Code) != tt.wantCode) {
				t.Fatalf("unexpected error: %+v", result.Error)
			}
			if strings.Contains(stdout.String()+stderr.String(), "synthetic-admin") ||
				strings.Contains(stdout.String()+stderr.String(), "SyntheticUser1!") ||
				strings.Contains(stdout.String()+stderr.String(), "synthetic-secret") {
				t.Fatal("secret leaked to output")
			}
			if !tt.profile && requests.Load() != 0 {
				t.Fatal("disabled command contacted device")
			}
		})
	}
}

func TestISAPIUserCredentialInput(t *testing.T) {
	admin, user, err := parseISAPIUserCredentials(`{"admin_password":"synthetic-admin","user_password":"SyntheticUser1!"}`)
	if err != nil || admin != "synthetic-admin" || user != "SyntheticUser1!" {
		t.Fatalf("valid synthetic credentials rejected: %v", err)
	}
	for _, input := range []string{
		`{}`,
		`{"admin_password":"x"}`,
		`{"admin_password":"x","user_password":"y","extra":"z"}`,
		`{"admin_password":"x","admin_password":"y","user_password":"z"}`,
		`{"admin_password":"x","user_password":"y"} {}`,
		`{"admin_password":"x","user_password":"y\nz"}`,
		`{"admin_password":"x","user_password":null}`,
		strings.Repeat("x", maxOneShotPasswordBytes+1),
		string([]byte{0xff}),
	} {
		admin, user, err := parseISAPIUserCredentials(input)
		if !errors.Is(err, errPasswordInputInvalid) || admin != "" || user != "" {
			t.Error("invalid credential input accepted")
		}
	}
}

func TestISAPIUserParsing(t *testing.T) {
	base := []string{"isapi-user", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05",
		"--username", "admin", "--password-stdin", "--user-id", "2", "--user-name", "synthetic", "--yes"}
	if _, err := ParseCLICommand(append(append([]string{}, base...), "--create")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		base,
		append(append([]string{}, base...), "--create", "--update"),
		append(append([]string{}, base...), "--update", "--yes"),
		{"isapi-user", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin", "--user-id", "../2", "--user-name", "synthetic", "--yes", "--update"},
		{"isapi-user", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin", "--user-id", "2", "--user-name", "synthetic", "--update"},
	} {
		if _, err := ParseCLICommand(args); err == nil {
			t.Error("invalid user options accepted")
		}
	}
	if _, err := ParseConsoleCommand("isapi-user --create"); err == nil {
		t.Fatal("interactive user mutation accepted")
	}
}
