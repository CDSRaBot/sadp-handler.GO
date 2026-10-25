package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewCreatesDirectoryAndRunHeader(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	now := time.Date(2026, 10, 25, 17, 20, 0, 0, time.FixedZone("UTC+5", 5*60*60))
	logger, handle, err := New(Config{Directory: dir, Level: "info", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer handle.Close()
	logger.Info("startup", "component", "test")
	data, err := os.ReadFile(filepath.Join(dir, "sadp-handler.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "\n===== logging started: 2026-10-25 17:20:00 +05:00 =====\n") {
		t.Fatalf("missing run header: %q", text)
	}
	if !strings.Contains(text, "msg=startup") {
		t.Fatalf("missing ordinary record: %q", text)
	}
}

func TestParseLevelAndFiltering(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug}, {"info", slog.LevelInfo}, {"warn", slog.LevelWarn},
		{"error", slog.LevelError}, {"", slog.LevelInfo},
	} {
		got, err := ParseLevel(tc.value)
		if err != nil || got != tc.want {
			t.Fatalf("ParseLevel(%q) = %v, %v; want %v", tc.value, got, err, tc.want)
		}
	}
	if _, err := ParseLevel("trace"); err == nil {
		t.Fatal("invalid level was accepted")
	}
	dir := t.TempDir()
	logger, handle, err := New(Config{Directory: dir, Level: "warn"})
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	logger.Info("hidden")
	logger.Warn("visible")
	data, err := os.ReadFile(filepath.Join(dir, "sadp-handler.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "msg=hidden") || !strings.Contains(string(data), "msg=visible") {
		t.Fatalf("level filtering failed: %q", data)
	}
}

func TestRotationHappensBeforeRecordWouldExceedLimit(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 25, 17, 20, 0, 0, time.UTC)
	header := "\n===== logging started: 2026-10-25 17:20:00 +00:00 =====\n"
	path := filepath.Join(dir, "sadp-handler.log")
	if err := os.WriteFile(path, make([]byte, int(MaxFileSize)-len(header)), 0o644); err != nil {
		t.Fatal(err)
	}
	logger, handle, err := New(Config{Directory: dir, Level: "info", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.Handler().Handle(nil, slog.NewRecord(now, slog.LevelInfo, "record", 0)); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	rotated, err := os.Stat(path + ".0001")
	if err != nil {
		t.Fatalf("rotated file missing: %v", err)
	}
	if rotated.Size() != int64(MaxFileSize) {
		t.Fatalf("rotated size = %d, want %d", rotated.Size(), MaxFileSize)
	}
	current, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if current.Size() > MaxFileSize {
		t.Fatalf("current file exceeds limit: %d", current.Size())
	}
}

func TestSafeAttrRedactsSensitiveValues(t *testing.T) {
	dir := t.TempDir()
	logger, handle, err := New(Config{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("operation",
		SafeAttr("password", "secret-password"),
		SafeAttr("challenge", "secret-challenge"),
		SafeAttr("payload", "<ProbeMatch>secret</ProbeMatch>"),
		SafeAttr("mode", "search"),
	)
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "sadp-handler.log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"secret-password", "secret-challenge", "ProbeMatch", "secret</"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("log leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "mode=search") || strings.Count(text, "[REDACTED]") != 3 {
		t.Fatalf("safe and redacted attributes missing: %s", text)
	}
}

func TestNewReturnsExplicitInitializationErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := New(Config{Directory: filepath.Join(file, "logs")}); err == nil {
		t.Fatal("directory initialization failure was hidden")
	}
	if _, _, err := New(Config{Directory: t.TempDir(), Filename: filepath.Join("nested", "file.log")}); err == nil {
		t.Fatal("invalid filename was accepted")
	}
	logger, handle, err := New(Config{Directory: t.TempDir(), MaxBytes: 0})
	if err != nil {
		t.Fatalf("zero size should select the default: %v", err)
	}
	if logger == nil {
		t.Fatal("zero-size configuration returned a nil logger")
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}
}
