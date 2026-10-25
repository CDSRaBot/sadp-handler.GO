package logging

import (
	"context"
	"errors"
	"log/slog"
)

func Event(logger *slog.Logger, level slog.Level, name string, attrs ...slog.Attr) error {
	if logger == nil {
		return errors.New("logger is nil")
	}
	if name == "" {
		return errors.New("log event name is empty")
	}
	logger.LogAttrs(context.Background(), level, name, attrs...)
	return nil
}
