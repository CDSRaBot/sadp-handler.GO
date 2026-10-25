package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/config"
)

func TestInitializeCreatesOneInjectedLoggerAndRunBoundary(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	dependencies, err := Initialize(config.Defaults(), dir)
	if err != nil {
		t.Fatalf("Initialize returned error: %v", err)
	}
	if dependencies.Logger == nil || dependencies.LogFile == nil {
		t.Fatalf("incomplete dependencies: %#v", dependencies)
	}
	if err := dependencies.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "sadp-handler.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "\n===== logging started:") {
		t.Fatalf("run header is not first: %q", data)
	}
	if !strings.Contains(string(data), "application.logger_initialized") {
		t.Fatalf("logger initialization event missing: %q", data)
	}
}

func TestInitializeMapsConfiguredLoggingLevel(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		settings := config.Defaults()
		settings.Logging.Level = level
		dependencies, err := Initialize(settings, t.TempDir())
		if err != nil {
			t.Fatalf("Initialize(%q) returned error: %v", level, err)
		}
		if err := dependencies.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInitializePropagatesLoggingFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Initialize(config.Defaults(), filepath.Join(file, "logs"))
	if err == nil || !strings.Contains(err.Error(), "initialize application logging") {
		t.Fatalf("error = %v, want explicit logging initialization error", err)
	}
}

func TestM8DoesNotAddLoggingSettingsToConfiguration(t *testing.T) {
	settings := config.Defaults()
	if settings.Logging.Level == "" {
		t.Fatal("M7 logging.level is missing")
	}
	if settings.Localization.Directory == "logs" {
		t.Fatal("logs directory leaked into localization settings")
	}
}
