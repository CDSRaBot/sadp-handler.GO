package app

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/config"
	"github.com/CDSRaBot/sadp-handler.GO/internal/discovery"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

type RuntimeConfig struct {
	UI           ui.RuntimeUIConfig
	Localization LocalizationRuntimeConfig
	Network      adapter.MulticastOptions
	Discovery    discovery.DiscoveryOptions
	Protocol     route.Strategy
	LogLevel     slog.Level
}

type LocalizationRuntimeConfig struct {
	FS fs.FS
}

type ServiceSet struct {
	UI    ui.Dependencies
	Route *route.Selector
}

type ApplicationDependencies struct {
	Config   RuntimeConfig
	Services ServiceSet
}

func Compose(settings config.Settings, baseDir string) (RuntimeConfig, error) {
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("resolve application directory: %w", err)
		}
	}
	root, err := filepath.Abs(baseDir)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("resolve application directory: %w", err)
	}
	_, err = confinedPath(root, settings.Localization.Directory)
	if err != nil {
		return RuntimeConfig{}, err
	}
	localizationFS, err := newLocalizationFS(root, settings.Localization.Directory)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("create localization filesystem: %w", err)
	}
	group, err := netip.ParseAddr(settings.Network.MulticastGroup)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("parse multicast group: %w", err)
	}
	cycleTimeout, err := time.ParseDuration(settings.Discovery.CycleTimeout)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("parse cycle timeout: %w", err)
	}
	var probeInterval time.Duration
	if settings.Discovery.ProbeInterval != "" {
		probeInterval, err = time.ParseDuration(settings.Discovery.ProbeInterval)
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("parse probe interval: %w", err)
		}
	}
	level, err := parseLogLevel(settings.Logging.Level)
	if err != nil {
		return RuntimeConfig{}, err
	}
	return RuntimeConfig{
		UI: ui.RuntimeUIConfig{
			Mode:     settings.UI.Mode,
			Format:   settings.UI.Format,
			Language: settings.UI.Language,
			Once:     settings.UI.Once,
		},
		Localization: LocalizationRuntimeConfig{FS: localizationFS},
		Network: adapter.MulticastOptions{
			Group: netip.AddrPortFrom(group, uint16(settings.Network.MulticastPort)),
			TTL:   settings.Network.MulticastTTL,
		},
		Discovery: discovery.DiscoveryOptions{
			Multicast:           adapter.MulticastOptions{Group: netip.AddrPortFrom(group, uint16(settings.Network.MulticastPort)), TTL: settings.Network.MulticastTTL},
			ProbeInterval:       probeInterval,
			CycleTimeout:        cycleTimeout,
			RepeatCount:         settings.Discovery.RepeatCount,
			MaxSADPDatagramSize: settings.Network.MaxSADPDatagramSize,
		},
		Protocol: route.Strategy(settings.Protocol.Strategy),
		LogLevel: level,
	}, nil
}

type localizationFS struct {
	root fs.FS
}

func newLocalizationFS(root, directory string) (fs.FS, error) {
	subtree, err := fs.Sub(os.DirFS(root), filepath.Clean(directory))
	if err != nil {
		return nil, err
	}
	return localizationFS{root: subtree}, nil
}

func (filesystem localizationFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	if name == "locales" {
		name = "."
	} else if strings.HasPrefix(name, "locales/") {
		name = strings.TrimPrefix(name, "locales/")
	}
	return filesystem.root.Open(name)
}

func BuildDependencies(runtime RuntimeConfig, services ServiceSet) ApplicationDependencies {
	services.UI.Config = runtime.UI
	services.UI.DiscoveryPolicy = ui.DiscoveryPolicy{
		ProbeInterval:       runtime.Discovery.ProbeInterval,
		CycleTimeout:        runtime.Discovery.CycleTimeout,
		RepeatCount:         runtime.Discovery.RepeatCount,
		MaxSADPDatagramSize: runtime.Discovery.MaxSADPDatagramSize,
	}
	return ApplicationDependencies{Config: runtime, Services: services}
}

func confinedPath(root, relative string) (string, error) {
	if relative == "" {
		return "", errors.New("localization directory is empty")
	}
	target := filepath.Clean(filepath.Join(root, relative))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		return "", errors.New("localization directory escapes application directory")
	}
	return target, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid logging level %q", value)
	}
}
