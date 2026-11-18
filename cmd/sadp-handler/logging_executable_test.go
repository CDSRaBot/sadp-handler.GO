package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecutableWritesCommandEvents(t *testing.T) {
	if os.Getenv("SADP_TEST_LOGGING_HELPER") == "1" {
		os.Args = []string{"sadp-handler", "--ui=no", "--format=json", "--once", "version"}
		os.Exit(run())
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExecutableWritesCommandEvents$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SADP_TEST_LOGGING_HELPER=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("executable failed: %v, output=%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "logs", "sadp-handler.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"application.logger_initialized", "ui.command_started", "ui.command_completed"} {
		if !strings.Contains(string(data), event) {
			t.Fatalf("missing %q in log", event)
		}
	}
	if strings.Contains(string(output), "ui.command_started") {
		t.Fatal("log event leaked into command output")
	}
}
