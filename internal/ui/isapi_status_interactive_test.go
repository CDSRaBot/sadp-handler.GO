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
	"sync/atomic"
	"testing"
)

func TestISAPIStatusInteractive(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/SDK/activateStatus" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected authenticated or mutating request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `<Status><Activated>false</Activated></Status>`)
	}))
	defer server.Close()
	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf("isapi-status --address %s --port %s", host, port)
	service := ISAPIKnownAddressStatusService{Client: server.Client()}

	t.Run("TUI", func(t *testing.T) {
		model := NewTUIModel(context.Background(), NewTUIState(), Dependencies{ISAPIStatus: service})
		model.console.SetValue(line)
		command := model.submitConsole()
		if command == nil {
			t.Fatal("status command not dispatched")
		}
		updated, _ := model.Update(command())
		view := updated.(TUIModel).View()
		if !strings.Contains(view, host+" state=inactive identity=unverified") {
			t.Fatalf("status or identity disclaimer missing: %q", view)
		}
	})
	for _, format := range []string{"text", "json"} {
		t.Run("REPL "+format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := RunCLIWithTerminalState(context.Background(), []string{"--ui=no", "--format=" + format},
				strings.NewReader(line+"\nquit\n"), &stdout, &stderr, true, false,
				Dependencies{ISAPIStatus: service})
			if exit != ExitSuccess {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
			}
			if format == "text" {
				if !strings.Contains(stdout.String(), host+" state=inactive identity=unverified") {
					t.Fatalf("status or disclaimer missing: %q", stdout.String())
				}
				return
			}
			var result CommandResult
			decoder := json.NewDecoder(&stdout)
			if err := decoder.Decode(&result); err != nil {
				t.Fatal(err)
			}
			if result.ISAPIStatus == nil || result.ISAPIStatus.Address != host ||
				result.ISAPIStatus.State != "inactive" || result.ISAPIStatus.IdentityVerified {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
	if requests.Load() != 3 {
		t.Fatalf("expected 3 read-only requests, got %d", requests.Load())
	}
}

func TestISAPIStatusConsoleRejectsInvalidInput(t *testing.T) {
	for _, line := range []string{
		"isapi-status",
		"isapi-status --address example.com",
		"isapi-status --address 192.0.2.1 --port 0",
		"isapi-status --address 192.0.2.1 --password-stdin",
		"isapi-status --address 192.0.2.1; quit",
	} {
		if _, err := ParseConsoleCommand(line); err == nil {
			t.Errorf("invalid input accepted: %s", line)
		}
	}
}
