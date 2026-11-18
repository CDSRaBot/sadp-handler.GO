package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestISAPIStatusExecutable(t *testing.T) {
	if os.Getenv("SADP_TEST_STATUS_HELPER") == "1" {
		os.Args = []string{"sadp-handler", "--ui=no", "--format=json", "--once", "isapi-status",
			"--address", "127.0.0.1", "--port", os.Getenv("SADP_TEST_PORT")}
		os.Exit(run())
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/SDK/activateStatus" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `<Status><Activated>false</Activated></Status>`)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestISAPIStatusExecutable$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "SADP_TEST_STATUS_HELPER=1", "SADP_TEST_PORT="+port)
	cmd.Stdin = strings.NewReader("")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("executable failed: %v, output=%s", err, output)
	}
	var result struct {
		OK     bool `json:"ok"`
		Status struct {
			Address          string `json:"address"`
			State            string `json:"state"`
			IdentityVerified bool   `json:"identity_verified"`
		} `json:"isapi_status"`
	}
	if err := json.Unmarshal(output, &result); err != nil || !result.OK || result.Status.Address != "127.0.0.1" ||
		result.Status.State != "inactive" || result.Status.IdentityVerified || !strings.Contains(string(output), `"identity_verified":false`) {
		t.Fatalf("unexpected result: %s (%v)", output, err)
	}
}
