package logging

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestModuleLoggingContractsUseStableSafeEvents(t *testing.T) {
	tests := []struct {
		module string
		event  string
		level  slog.Level
	}{
		{"M0 adapter", "interface.enumeration", slog.LevelInfo},
		{"M1 network", "datagram.rejected", slog.LevelWarn},
		{"M2 parser", "advertisement.rejected", slog.LevelWarn},
		{"M3 logic", "workflow.completed", slog.LevelInfo},
		{"M4 UI", "command.dispatched", slog.LevelInfo},
		{"M5 localization", "catalog.load_failed", slog.LevelWarn},
		{"M6 discovery", "session.cancelled", slog.LevelInfo},
		{"M7 configuration", "settings.validation_failed", slog.LevelError},
	}
	for _, tc := range tests {
		t.Run(tc.module, func(t *testing.T) {
			handler := &recordingHandler{}
			logger := slog.New(handler)
			if err := Event(logger, tc.level, tc.event, SafeAttr("stage", "test"), SafeAttr("count", 1)); err != nil {
				t.Fatal(err)
			}
			if handler.record.Level != tc.level || handler.record.Message != tc.event {
				t.Fatalf("record = %#v, want %s at %s", handler.record, tc.event, tc.level)
			}
			if handler.record.NumAttrs() != 2 {
				t.Fatalf("attribute count = %d, want 2", handler.record.NumAttrs())
			}
		})
	}
}

func TestModuleLoggingContractsRedactForbiddenValues(t *testing.T) {
	handler := &recordingHandler{}
	logger := slog.New(handler)
	forbidden := []string{"password-value", "derived-hash", "challenge-value", "<xml>secret</xml>", `{"secret":"value"}`, "payload-value"}
	attrs := []slog.Attr{
		SafeAttr("password", forbidden[0]),
		SafeAttr("hash", forbidden[1]),
		SafeAttr("challenge", forbidden[2]),
		SafeAttr("xml", forbidden[3]),
		SafeAttr("json_config", forbidden[4]),
		SafeAttr("payload", forbidden[5]),
	}
	if err := Event(logger, slog.LevelWarn, "security.rejected", attrs...); err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	handler.record.Attrs(func(attr slog.Attr) bool {
		text.WriteString(attr.Value.String())
		return true
	})
	for _, value := range forbidden {
		if strings.Contains(text.String(), value) {
			t.Fatalf("forbidden value %q was retained", value)
		}
	}
}

func TestEventRejectsInvalidLoggerOrName(t *testing.T) {
	if err := Event(nil, slog.LevelInfo, "event"); err == nil {
		t.Fatal("nil logger was accepted")
	}
	if err := Event(slog.Default(), slog.LevelInfo, ""); err == nil {
		t.Fatal("empty event name was accepted")
	}
}

func TestEventReturnsHandlerError(t *testing.T) {
	expected := errors.New("log write failed")
	logger := slog.New(errorHandler{err: expected})

	if err := Event(logger, slog.LevelError, "event"); !errors.Is(err, expected) {
		t.Fatalf("Event() error = %v, want handler error", err)
	}
}

type recordingHandler struct {
	record slog.Record
}

type errorHandler struct {
	err error
}

func (h errorHandler) Enabled(context.Context, slog.Level) bool  { return true }
func (h errorHandler) Handle(context.Context, slog.Record) error { return h.err }
func (h errorHandler) WithAttrs([]slog.Attr) slog.Handler        { return h }
func (h errorHandler) WithGroup(string) slog.Handler             { return h }

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, record slog.Record) error {
	h.record = record.Clone()
	return nil
}
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }
