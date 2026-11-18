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

func TestISAPIDiscoveryExecutable(t *testing.T) {
	if os.Getenv("SADP_TEST_DISCOVERY_HELPER") == "1" {
		os.Args = []string{"sadp-handler", "--ui=no", "--format=json", "--once", "isapi-discover",
			"--address", "127.0.0.1", "--port", os.Getenv("SADP_TEST_PORT"),
			"--device", "00:01:02:03:04:05", "--username", "admin", "--password-stdin"}
		os.Exit(run())
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ISAPI/System/deviceInfo" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="test", qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress><model>Synthetic</model></DeviceInfo>`)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestISAPIDiscoveryExecutable$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "SADP_TEST_DISCOVERY_HELPER=1", "SADP_TEST_PORT="+port)
	cmd.Stdin = strings.NewReader("synthetic-password\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("executable failed: %v, output=%s", err, output)
	}
	if strings.Contains(string(output), "synthetic-password") {
		t.Fatal("password leaked")
	}
	var result struct {
		OK     bool `json:"ok"`
		Device struct {
			MAC   string `json:"mac"`
			Model string `json:"model"`
		} `json:"isapi_device"`
	}
	if err := json.Unmarshal(output, &result); err != nil || !result.OK || result.Device.MAC != "00:01:02:03:04:05" || result.Device.Model != "Synthetic" {
		t.Fatalf("unexpected result: %s (%v)", output, err)
	}
}
