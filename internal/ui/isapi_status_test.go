package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseISAPIStatus(t *testing.T) {
	command, err := ParseCLICommand([]string{"isapi-status", "--address", "192.0.2.1"})
	if err != nil || command.(ISAPIStatusCommand).Port != 80 {
		t.Fatalf("default port: %v, %v", command, err)
	}
	command, err = ParseCLICommand([]string{"isapi-status", "--https", "--address", "192.0.2.1"})
	if err != nil || command.(ISAPIStatusCommand).Port != 443 {
		t.Fatalf("HTTPS port: %v, %v", command, err)
	}
	for _, args := range [][]string{
		{"isapi-status"},
		{"isapi-status", "--address", "example.com"},
		{"isapi-status", "--address", "224.0.0.1"},
		{"isapi-status", "--address", "fe80::1"},
		{"isapi-status", "--address", "192.0.2.1", "--port", "0"},
		{"isapi-status", "--address", "192.0.2.1", "--port", "65536"},
		{"isapi-status", "--address", "192.0.2.1", "--https", "--https"},
		{"isapi-status", "--address", "192.0.2.1", "--password-stdin"},
	} {
		if _, err := ParseCLICommand(args); err == nil {
			t.Errorf("accepted invalid command: %v", args)
		}
	}
}

func TestISAPIStatusOneShot(t *testing.T) {
	for _, tt := range []struct {
		name, response, format, state, code string
		tls                                 bool
		wantExit                            int
	}{
		{"active text", `<Status><Activated>true</Activated></Status>`, "text", "active", "", false, ExitSuccess},
		{"inactive JSON", `<Status><Activated>0</Activated></Status>`, "json", "inactive", "", false, ExitSuccess},
		{"HTTPS text", `<Status><Activated>false</Activated></Status>`, "text", "inactive", "", true, ExitSuccess},
		{"malformed", `<Status><Activated>true</Activated><Activated>false</Activated></Status>`, "json", "", "malformed_response", false, ExitOperational},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/SDK/activateStatus" || r.Header.Get("Authorization") != "" {
					t.Errorf("unexpected authenticated or mutating request")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_, _ = io.WriteString(w, tt.response)
			})
			server := httptest.NewUnstartedServer(handler)
			if tt.tls {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			host, port, err := net.SplitHostPort(server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"--ui=no", "--format=" + tt.format, "--once", "isapi-status", "--address", host, "--port", port}
			if tt.tls {
				args = append(args, "--https")
			}
			deps := Dependencies{ISAPIStatus: ISAPIKnownAddressStatusService{Client: server.Client()}}
			var output, diagnostics bytes.Buffer
			exit := RunCLIWithTerminalState(context.Background(), args, strings.NewReader(""), &output, &diagnostics, false, false, deps)
			if exit != tt.wantExit {
				t.Fatalf("exit=%d output=%s diagnostic=%s", exit, output.String(), diagnostics.String())
			}
			if tt.format == "text" {
				want := host + " state=" + tt.state + " identity=unverified"
				if strings.TrimSpace(output.String()) != want {
					t.Fatalf("text output=%q, want %q", output.String(), want)
				}
				return
			}
			var result CommandResult
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if tt.code != "" {
				if result.Error == nil || string(result.Error.Code) != tt.code || result.ISAPIStatus != nil {
					t.Fatalf("unexpected error: %+v", result)
				}
			} else if result.ISAPIStatus == nil || result.ISAPIStatus.Address != host ||
				result.ISAPIStatus.State != tt.state || result.ISAPIStatus.IdentityVerified {
				t.Fatalf("unexpected status: %+v", result)
			}
			if tt.code == "" && !strings.Contains(output.String(), `"identity_verified":false`) {
				t.Fatalf("identity disclaimer missing: %s", output.String())
			}
		})
	}
}

type isapiStatusServiceFunc func(context.Context, ISAPIStatusCommand) (ISAPIStatusResult, error)

func (f isapiStatusServiceFunc) ReadISAPIStatus(ctx context.Context, cmd ISAPIStatusCommand) (ISAPIStatusResult, error) {
	return f(ctx, cmd)
}

func TestISAPIStatusRejectsInteractiveAndInvalidServiceResults(t *testing.T) {
	command, err := ParseCLICommand([]string{"isapi-status", "--address", "192.0.2.1"})
	if err != nil {
		t.Fatal(err)
	}
	service := isapiStatusServiceFunc(func(context.Context, ISAPIStatusCommand) (ISAPIStatusResult, error) {
		return ISAPIStatusResult{Address: "192.0.2.1", State: "active", IdentityVerified: true}, nil
	})
	result, err := DispatchCommand(context.Background(), command, Dependencies{Config: RuntimeUIConfig{Mode: "no"}, ISAPIStatus: service})
	if err == nil || result.Error == nil || result.Error.Code != FailureUnsupported {
		t.Fatalf("interactive mode accepted: %+v, %v", result, err)
	}
	result, err = DispatchCommand(context.Background(), command, Dependencies{Config: RuntimeUIConfig{Mode: "no", Once: true}, ISAPIStatus: service})
	if err == nil || result.ISAPIStatus != nil || result.Error == nil {
		t.Fatalf("invalid service result accepted: %+v, %v", result, err)
	}
}
