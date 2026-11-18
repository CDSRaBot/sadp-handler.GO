package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

func TestISAPIUserExecutableRemainsDisabled(t *testing.T) {
	if os.Getenv("SADP_TEST_USER_HELPER") == "1" {
		os.Args = []string{"sadp-handler", "--ui=no", "--format=json", "--once", "isapi-user",
			"--address", "127.0.0.1", "--port", os.Getenv("SADP_TEST_PORT"), "--device", "00:01:02:03:04:05",
			"--username", "admin", "--password-stdin", "--user-id", "2", "--user-name", "synthetic", "--update", "--yes"}
		os.Exit(run())
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestISAPIUserExecutableRemainsDisabled$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "SADP_TEST_USER_HELPER=1", "SADP_TEST_PORT="+port)
	cmd.Stdin = reader
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("disabled user operation waited for credential input")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("unexpected exit: %v, output=%s", err, output)
	}
	var result struct {
		OK   bool `json:"ok"`
		User struct {
			Attempted bool `json:"attempted"`
			Verified  bool `json:"verified"`
		} `json:"isapi_user"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(output, &result); err != nil || result.OK || result.Error.Code != "unsupported" ||
		result.User.Attempted || result.User.Verified || requests.Load() != 0 {
		t.Fatalf("unexpected result: %s (%v), requests=%d", output, err, requests.Load())
	}
}
