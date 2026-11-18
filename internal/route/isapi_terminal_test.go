package route

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParseISAPITerminalDiscovery(t *testing.T) {
	const root = `<DiscoveryMode xmlns="http://www.isapi.org/ver20/XMLSchema" version="2.0">`
	state, err := parseISAPITerminalDiscovery([]byte(root + `<ONVIFDiscoveryMode>discoverable</ONVIFDiscoveryMode>` +
		`<ISDiscoveryMode>nonDiscoverable</ISDiscoveryMode><PcapMode>discoverable</PcapMode>` +
		`<SADP>nonDiscoverable</SADP><Extension><SADP>ignored</SADP></Extension></DiscoveryMode>`))
	if err != nil || state.ONVIFDiscoveryMode == nil || *state.ONVIFDiscoveryMode != "discoverable" ||
		state.ISDiscoveryMode == nil || *state.ISDiscoveryMode != "nonDiscoverable" ||
		state.PcapMode == nil || state.SADP == nil {
		t.Fatalf("unexpected state=%+v err=%v", state, err)
	}
	partial, err := parseISAPITerminalDiscovery([]byte(root + `<SADP>discoverable</SADP></DiscoveryMode>`))
	if err != nil || partial.SADP == nil || partial.ONVIFDiscoveryMode != nil {
		t.Fatalf("optional fields lost: %+v %v", partial, err)
	}
	for _, payload := range []string{
		`<DiscoveryMode version="2.0"><SADP>discoverable</SADP></DiscoveryMode>`,
		`<DiscoveryMode xmlns="http://www.isapi.org/ver20/XMLSchema"><SADP>discoverable</SADP></DiscoveryMode>`,
		`<DiscoveryMode xmlns="http://www.isapi.org/ver20/XMLSchema" version="" version="2.0"><SADP>discoverable</SADP></DiscoveryMode>`,
		root + `<SADP>discoverable</SADP><SADP>nonDiscoverable</SADP></DiscoveryMode>`,
		root + `<SADP xmlns="">discoverable</SADP></DiscoveryMode>`,
		root + `<SADP>true</SADP></DiscoveryMode>`,
		root + `<SADP><value>discoverable</value></SADP></DiscoveryMode>`,
		root + `<Extension/></DiscoveryMode>`,
		root + `<SADP>discoverable</SADP></DiscoveryMode>trailing`,
		root + `<SADP>discoverable</SADP></DiscoveryMode>` + root + `</DiscoveryMode>`,
	} {
		if _, err := parseISAPITerminalDiscovery([]byte(payload)); err == nil {
			t.Errorf("malformed terminal response accepted")
		}
	}
}

func TestISAPITerminalDiscoveryAuthenticatedRead(t *testing.T) {
	for _, tt := range []struct {
		name         string
		wrongMAC     bool
		malformed    bool
		wantErr      ErrorKind
		wantTerminal int32
	}{
		{name: "verified", wantTerminal: 1},
		{name: "identity mismatch", wrongMAC: true, wantErr: ErrorVerification},
		{name: "malformed state", malformed: true, wantErr: ErrorMalformed, wantTerminal: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var terminalRequests atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Error("terminal read performed a mutation")
				}

				user, password, ok := r.BasicAuth()
				if !ok || user != "admin" || password != "synthetic-password" {
					w.Header().Set("WWW-Authenticate", `Basic realm="synthetic"`)
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
					terminalRequests.Add(1)
					value := "discoverable"
					if tt.malformed {
						value = "unknown"
					}
					_, _ = fmt.Fprintf(w, `<DiscoveryMode xmlns="%s" version="2.0"><SADP>%s</SADP></DiscoveryMode>`, isapiXMLNamespace, value)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Client: server.Client(), Username: "admin",
				Password: "synthetic-password", AllowBasic: true}
			state, err := transport.ReadTerminalDiscovery(context.Background(), serverTarget(t, server), "2")
			if tt.wantErr == "" {
				if err != nil || state.SADP == nil || *state.SADP != "discoverable" {
					t.Fatalf("state=%+v err=%v", state, err)
				}
			} else {
				var routeErr *Error
				if !errors.As(err, &routeErr) || routeErr.Kind != tt.wantErr || strings.Contains(err.Error(), "synthetic-password") {
					t.Fatalf("error=%v, want %s", err, tt.wantErr)
				}
			}

			if terminalRequests.Load() != tt.wantTerminal {
				t.Fatalf("terminal requests=%d", terminalRequests.Load())
			}
		})
	}
}

func TestISAPITerminalRejectsUnauthenticatedResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ISAPI/System/deviceInfo":
			if _, _, ok := r.BasicAuth(); !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="synthetic"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = fmt.Fprint(w, `<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress></DeviceInfo>`)
		case "/ISAPI/Publish/TerminalMgr/terminals/discovery/2":
			_, _ = fmt.Fprintf(w, `<DiscoveryMode xmlns="%s" version="2.0"><SADP>discoverable</SADP></DiscoveryMode>`, isapiXMLNamespace)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	transport := &ISAPIHTTPTransport{BaseURL: server.URL, Client: server.Client(), Username: "admin",
		Password: "synthetic-password", AllowBasic: true}
	_, err := transport.ReadTerminalDiscovery(context.Background(), serverTarget(t, server), "2")
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorAuthentication {
		t.Fatalf("unauthenticated response accepted: %v", err)
	}
}
