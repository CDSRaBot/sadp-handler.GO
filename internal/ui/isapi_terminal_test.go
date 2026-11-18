package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func TestISAPITerminalOneShot(t *testing.T) {
	for _, tt := range []struct {
		name       string
		wrongMAC   bool
		terminalOK bool
		wantExit   int
		wantCode   string
	}{
		{"verified", false, true, ExitSuccess, ""},
		{"wrong identity", true, true, ExitOperational, "verification"},
		{"bad XML", false, false, ExitOperational, "malformed_response"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			terminalRequests := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method %s", r.Method)
				}
				user, password, ok := r.BasicAuth()
				if !ok || user != "admin" || password != "synthetic-password" {
					w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				switch r.URL.Path {
				case "/ISAPI/System/deviceInfo":
					mac := "00:01:02:03:04:05"
					if tt.wrongMAC {
						mac = "00:01:02:03:04:06"
					}
					_, _ = fmt.Fprintf(w, "<DeviceInfo><macAddress>%s</macAddress></DeviceInfo>", mac)
				case "/ISAPI/Publish/TerminalMgr/terminals/discovery/2":
					terminalRequests++
					value := "discoverable"
					if !tt.terminalOK {
						value = "bad"
					}
					_, _ = fmt.Fprintf(w, `<DiscoveryMode xmlns="http://www.isapi.org/ver20/XMLSchema" version="2.0"><SADP>%s</SADP></DiscoveryMode>`, value)
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
			deps := Dependencies{ISAPITerminal: ISAPIKnownAddressTerminalService{
				Passwords: oneShotPasswordProvider{reader: io.NopCloser(strings.NewReader("synthetic-password\n"))}, Client: server.Client(),
			}}
			args := []string{"--ui=no", "--format=json", "--once", "isapi-terminal", "--address", host,
				"--port", port, "--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin",
				"--https", "--basic", "--terminal-id", "2"}
			var out, diagnostics bytes.Buffer
			exit := RunCLIWithTerminalState(context.Background(), args, strings.NewReader(""), &out, &diagnostics, false, false, deps)
			if exit != tt.wantExit {
				t.Fatalf("exit=%d output=%s diagnostics=%s", exit, out.String(), diagnostics.String())
			}
			if strings.Contains(out.String()+diagnostics.String(), "synthetic-password") {
				t.Fatal("password leaked")
			}
			var result CommandResult
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if tt.wantCode != "" {
				if result.Error == nil || string(result.Error.Code) != tt.wantCode || result.ISAPITerminal != nil {
					t.Fatalf("unexpected failure: %+v", result)
				}
			} else if result.ISAPITerminal == nil || result.ISAPITerminal.TerminalID != "2" ||
				result.ISAPITerminal.Discovery.SADP == nil || *result.ISAPITerminal.Discovery.SADP != "discoverable" {
				t.Fatalf("unexpected result: %+v", result)
			}
			if tt.wrongMAC && terminalRequests != 0 {
				t.Fatal("terminal read before identity verification")
			}
		})
	}
}

func TestParseISAPITerminal(t *testing.T) {
	base := []string{"isapi-terminal", "--address", "192.0.2.1", "--device", "00:01:02:03:04:05",
		"--username", "admin", "--password-stdin"}
	for _, id := range []string{"", ".", "..", "../2", "capabilities", "2?x=3"} {
		args := append(append([]string{}, base...), "--terminal-id", id)
		if _, err := ParseCLICommand(args); err == nil {
			t.Errorf("accepted invalid terminal ID %q", id)
		}
	}
	if _, err := ParseCLICommand(append(append([]string{}, base...), "--terminal-id", "2", "--terminal-id", "3")); err == nil {
		t.Fatal("accepted duplicate terminal ID")
	}
	command, err := ParseCLICommand(append(append([]string{}, base...), "--terminal-id", "2"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := DispatchCommand(context.Background(), command, Dependencies{Config: RuntimeUIConfig{Mode: "no"}})
	if err == nil || result.Error == nil || result.Error.Code != FailureUnsupported {
		t.Fatalf("interactive mode was not rejected: %+v, %v", result, err)
	}
}

func TestISAPIOperationTextResultsAreIndependent(t *testing.T) {
	value := "discoverable"
	tests := []struct {
		result CommandResult
		want   string
	}{
		{CommandResult{ISAPINetwork: &ISAPINetworkResult{Address: "192.0.2.1", MAC: "00:01:02:03:04:05", Attempted: true}}, "attempted=true verified=false restart_required=false"},
		{CommandResult{ISAPIUser: &ISAPIUserResult{Address: "192.0.2.1", MAC: "00:01:02:03:04:05", ID: "2", Attempted: true}}, "id=2 attempted=true verified=false"},
		{CommandResult{ISAPITerminal: &ISAPITerminalResult{TerminalID: "2", Discovery: route.ISAPITerminalDiscovery{SADP: &value}}}, "terminal_id=2 sadp=discoverable"},
	}
	for _, tt := range tests {
		var output bytes.Buffer
		if err := writeResult(&output, FormatText, tt.result); err != nil || !strings.Contains(output.String(), tt.want) {
			t.Fatalf("text output %q, err=%v, want %q", output.String(), err, tt.want)
		}
	}
}
