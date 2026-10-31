package logging

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

func Event(logger *slog.Logger, level slog.Level, name string, attrs ...slog.Attr) error {
	if logger == nil {
		return errors.New("logger is nil")
	}
	if name == "" {
		return errors.New("log event name is empty")
	}
	handler := logger.Handler()
	if !handler.Enabled(context.Background(), level) {
		return nil
	}
	record := slog.NewRecord(time.Now(), level, name, 0)
	record.AddAttrs(attrs...)
	return handler.Handle(context.Background(), record)
}
