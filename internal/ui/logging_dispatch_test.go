package ui

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type failingLogHandler struct {
	calls  int
	failAt int
}

func (h *failingLogHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *failingLogHandler) Handle(context.Context, slog.Record) error {
	h.calls++
	if h.calls == h.failAt {
		return errors.New("synthetic log failure")
	}
	return nil
}
func (h *failingLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *failingLogHandler) WithGroup(string) slog.Handler      { return h }

func TestCommandLoggingFailuresAreExplicit(t *testing.T) {
	for _, tt := range []struct {
		failAt      int
		wantVersion string
		wantMessage string
	}{
		{1, "", "command was not run"},
		{2, "synthetic-version", "after command execution"},
	} {
		handler := &failingLogHandler{failAt: tt.failAt}
		result, err := DispatchCommand(context.Background(), VersionCommand{}, Dependencies{
			Logger: slog.New(handler), Version: "synthetic-version",
		})
		if err == nil || result.OK || result.Version != tt.wantVersion || result.Error == nil ||
			!strings.Contains(result.Error.Message, tt.wantMessage) {
			t.Fatalf("failAt=%d result=%+v err=%v", tt.failAt, result, err)
		}
	}
}
