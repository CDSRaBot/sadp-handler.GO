package app

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/CDSRaBot/sadp-handler.GO/internal/config"
	"github.com/CDSRaBot/sadp-handler.GO/internal/logging"
)

type Dependencies struct {
	Settings config.Settings
	Logger   *slog.Logger
	LogFile  *logging.Handle
}

func Initialize(settings config.Settings, directory string) (Dependencies, error) {
	if directory == "" {
		directory = "logs"
	}
	logger, logFile, err := logging.New(logging.Config{
		Directory: directory,
		Level:     settings.Logging.Level,
	})
	if err != nil {
		return Dependencies{}, fmt.Errorf("initialize application logging: %w", err)
	}
	if logger == nil || logFile == nil {
		return Dependencies{}, errors.New("logging initialization returned incomplete dependencies")
	}
	if err := logging.Event(logger, slog.LevelInfo, "application.logger_initialized",
		logging.SafeAttr("component", "M8"),
		logging.SafeAttr("level", settings.Logging.Level),
	); err != nil {
		closeErr := logFile.Close()
		return Dependencies{}, errors.Join(
			fmt.Errorf("log application initialization: %w", err),
			func() error {
				if closeErr == nil {
					return nil
				}
				return fmt.Errorf("close application log: %w", closeErr)
			}(),
		)
	}
	return Dependencies{Settings: settings, Logger: logger, LogFile: logFile}, nil
}

func (d Dependencies) Close() error {
	if d.LogFile == nil {
		return nil
	}
	return d.LogFile.Close()
}
