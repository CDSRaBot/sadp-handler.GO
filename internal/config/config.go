// Package config loads and validates the external runtime configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/i18n"
)

const (
	defaultSettingsPath     = "settings.json"
	defaultMaxDatagramSize  = 1500
	defaultMulticastGroup   = "239.255.255.250"
	defaultMulticastPort    = 37020
	defaultMulticastTTL     = 64
	defaultLocalizationDir  = "locales"
	defaultCycleTimeout     = 5 * time.Second
	defaultRepeatCount      = 1
	defaultLanguage         = "en"
	defaultLoggingLevel     = "info"
	defaultProtocolStrategy = "sadp_first"
	defaultUIMode           = "yes"
	defaultUIFormat         = "text"
)

type UISettings struct {
	Mode     string
	Format   string
	Language string
	Once     bool
}

type LocalizationSettings struct {
	Directory string
}

type NetworkSettings struct {
	MulticastGroup      string
	MulticastPort       int
	MulticastTTL        int
	MaxSADPDatagramSize int
}

type DiscoverySettings struct {
	ProbeInterval string
	CycleTimeout  string
	RepeatCount   int
}

type LoggingSettings struct {
	Level string
}

type ProtocolSettings struct {
	Strategy string
}

type Settings struct {
	UI           UISettings
	Localization LocalizationSettings
	Network      NetworkSettings
	Discovery    DiscoverySettings
	Protocol     ProtocolSettings
	Logging      LoggingSettings
}

type ConfigurationErrorKind string

const (
	ConfigurationReadError               ConfigurationErrorKind = "read"
	ConfigurationDecodeError             ConfigurationErrorKind = "decode"
	ConfigurationUnknownKeyError         ConfigurationErrorKind = "unknown_key"
	ConfigurationMissingCommentError     ConfigurationErrorKind = "missing_comment"
	ConfigurationInvalidValueError       ConfigurationErrorKind = "invalid_value"
	ConfigurationInvalidCombinationError ConfigurationErrorKind = "invalid_combination"
)

type ConfigurationError struct {
	Kind  ConfigurationErrorKind
	Field string
	Err   error
}

func (e *ConfigurationError) Error() string {
	if e == nil {
		return ""
	}
	message := string(e.Kind)
	if e.Field != "" {
		message += " (" + e.Field + ")"
	}
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e *ConfigurationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Defaults() Settings {
	return Settings{
		UI: UISettings{
			Mode:     defaultUIMode,
			Format:   defaultUIFormat,
			Language: defaultLanguage,
		},
		Localization: LocalizationSettings{Directory: defaultLocalizationDir},
		Network: NetworkSettings{
			MulticastGroup:      defaultMulticastGroup,
			MulticastPort:       defaultMulticastPort,
			MulticastTTL:        defaultMulticastTTL,
			MaxSADPDatagramSize: defaultMaxDatagramSize,
		},
		Discovery: DiscoverySettings{
			CycleTimeout: defaultCycleTimeout.String(),
			RepeatCount:  defaultRepeatCount,
		},
		Logging:  LoggingSettings{Level: defaultLoggingLevel},
		Protocol: ProtocolSettings{Strategy: defaultProtocolStrategy},
	}
}

type rawSettings struct {
	Comments     map[string]string `json:"_comments"`
	UI           rawUISettings     `json:"ui"`
	Localization rawLocalization   `json:"localization"`
	Network      rawNetwork        `json:"network"`
	Discovery    rawDiscovery      `json:"discovery"`
	Protocol     rawProtocol       `json:"protocol"`
	Logging      rawLogging        `json:"logging"`
}

type rawUISettings struct {
	Mode     *string `json:"mode"`
	Format   *string `json:"format"`
	Language *string `json:"language"`
	Once     *bool   `json:"once"`
}

type rawLocalization struct {
	Directory *string `json:"directory"`
}

type rawNetwork struct {
	MulticastGroup      *string `json:"multicast_group"`
	MulticastPort       *int    `json:"multicast_port"`
	MulticastTTL        *int    `json:"multicast_ttl"`
	MaxSADPDatagramSize *int    `json:"max_sadp_datagram_size"`
}

type rawDiscovery struct {
	ProbeInterval *string `json:"probe_interval"`
	CycleTimeout  *string `json:"cycle_timeout"`
	RepeatCount   *int    `json:"repeat_count"`
}

type rawLogging struct {
	Level *string `json:"level"`
}

type rawProtocol struct {
	Strategy *string `json:"strategy"`
}

var requiredComments = map[string]struct{}{
	"ui.mode":                        {},
	"ui.format":                      {},
	"ui.language":                    {},
	"ui.once":                        {},
	"localization.directory":         {},
	"network.multicast_group":        {},
	"network.multicast_port":         {},
	"network.multicast_ttl":          {},
	"network.max_sadp_datagram_size": {},
	"discovery.probe_interval":       {},
	"discovery.cycle_timeout":        {},
	"discovery.repeat_count":         {},
	"logging.level":                  {},
	"protocol.strategy":              {},
}

func Load(path string) (Settings, error) {
	if path == "" {
		path = defaultSettingsPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Defaults(), nil
		}
		return Settings{}, &ConfigurationError{
			Kind: ConfigurationReadError,
			Err:  err,
		}
	}

	var raw rawSettings
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return Settings{}, &ConfigurationError{
			Kind: ConfigurationDecodeError,
			Err:  err,
		}
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return Settings{}, &ConfigurationError{
			Kind: ConfigurationDecodeError,
			Err:  errors.New("trailing JSON data"),
		}
	} else if !errors.Is(err, io.EOF) {
		return Settings{}, &ConfigurationError{
			Kind: ConfigurationDecodeError,
			Err:  err,
		}
	}

	if raw.Comments == nil {
		return Settings{}, &ConfigurationError{
			Kind:  ConfigurationMissingCommentError,
			Field: "_comments",
			Err:   errors.New("comment metadata is required"),
		}
	}
	if err := validateComments(raw.Comments); err != nil {
		return Settings{}, err
	}

	settings := Defaults()
	if raw.UI.Mode != nil {
		settings.UI.Mode = *raw.UI.Mode
	}
	if raw.UI.Format != nil {
		settings.UI.Format = *raw.UI.Format
	}
	if raw.UI.Language != nil {
		settings.UI.Language = *raw.UI.Language
	}
	if raw.UI.Once != nil {
		settings.UI.Once = *raw.UI.Once
	}
	if raw.Localization.Directory != nil {
		settings.Localization.Directory = *raw.Localization.Directory
	}
	if raw.Network.MulticastGroup != nil {
		settings.Network.MulticastGroup = *raw.Network.MulticastGroup
	}
	if raw.Network.MulticastPort != nil {
		settings.Network.MulticastPort = *raw.Network.MulticastPort
	}
	if raw.Network.MulticastTTL != nil {
		settings.Network.MulticastTTL = *raw.Network.MulticastTTL
	}
	if raw.Network.MaxSADPDatagramSize != nil {
		settings.Network.MaxSADPDatagramSize = *raw.Network.MaxSADPDatagramSize
		if settings.Network.MaxSADPDatagramSize == 0 {
			settings.Network.MaxSADPDatagramSize = defaultMaxDatagramSize
		}
	}
	if raw.Discovery.ProbeInterval != nil {
		settings.Discovery.ProbeInterval = *raw.Discovery.ProbeInterval
	}
	if raw.Discovery.CycleTimeout != nil {
		settings.Discovery.CycleTimeout = *raw.Discovery.CycleTimeout
	}
	if raw.Discovery.RepeatCount != nil {
		settings.Discovery.RepeatCount = *raw.Discovery.RepeatCount
	}
	if raw.Logging.Level != nil {
		settings.Logging.Level = *raw.Logging.Level
	}
	if raw.Protocol.Strategy != nil {
		settings.Protocol.Strategy = *raw.Protocol.Strategy
	}

	if err := validate(settings); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func validateComments(comments map[string]string) error {
	for field := range comments {
		if _, ok := requiredComments[field]; !ok {
			return &ConfigurationError{
				Kind:  ConfigurationUnknownKeyError,
				Field: "_comments." + field,
				Err:   errors.New("unknown comment key"),
			}
		}
	}
	for field := range requiredComments {
		if strings.TrimSpace(comments[field]) == "" {
			return &ConfigurationError{
				Kind:  ConfigurationMissingCommentError,
				Field: "_comments." + field,
				Err:   errors.New("comment is required"),
			}
		}
	}
	return nil
}

func validate(settings Settings) error {
	if settings.UI.Mode != "yes" && settings.UI.Mode != "no" {
		return invalidValue("ui.mode", "must be yes or no")
	}
	if settings.UI.Format != "text" && settings.UI.Format != "json" {
		return invalidValue("ui.format", "must be text or json")
	}
	if err := i18n.ValidateLocaleCode(settings.UI.Language); err != nil {
		return invalidValue("ui.language", "must be a valid locale code")
	}
	if settings.UI.Once && settings.UI.Mode != "no" {
		return invalidCombination("ui.once", "requires ui.mode=no")
	}
	if err := validateRelativePath("localization.directory", settings.Localization.Directory); err != nil {
		return err
	}

	group, err := netip.ParseAddr(settings.Network.MulticastGroup)
	if err != nil || !group.Is4() || !group.IsMulticast() {
		return invalidValue("network.multicast_group", "must be a multicast IPv4 address")
	}
	if settings.Network.MulticastPort < 1 || settings.Network.MulticastPort > 65535 {
		return invalidValue("network.multicast_port", "must be between 1 and 65535")
	}
	if settings.Network.MulticastTTL < 1 || settings.Network.MulticastTTL > 255 {
		return invalidValue("network.multicast_ttl", "must be between 1 and 255")
	}
	if settings.Network.MaxSADPDatagramSize < 1 || settings.Network.MaxSADPDatagramSize > 65507 {
		return invalidValue("network.max_sadp_datagram_size", "must be 0 or between 1 and 65507")
	}

	probeInterval, err := parseDuration("discovery.probe_interval", settings.Discovery.ProbeInterval)
	if err != nil {
		return err
	}
	cycleTimeout, err := parseDuration("discovery.cycle_timeout", settings.Discovery.CycleTimeout)
	if err != nil {
		return err
	}
	if probeInterval > 0 {
		if cycleTimeout != 0 || settings.Discovery.RepeatCount != 0 {
			return invalidCombination("discovery", "continuous mode requires cycle_timeout=0 and repeat_count=0")
		}
	} else if cycleTimeout <= 0 || settings.Discovery.RepeatCount < 1 || settings.Discovery.RepeatCount > 100 {
		return invalidCombination("discovery", "bounded mode requires positive cycle_timeout and repeat_count 1..100")
	}
	if settings.Protocol.Strategy != "sadp_first" && settings.Protocol.Strategy != "isapi_only" {
		return invalidValue("protocol.strategy", "must be sadp_first or isapi_only")
	}
	switch settings.Logging.Level {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return invalidValue("logging.level", "must be debug, info, warn, or error")
	}
}

func validateRelativePath(field, value string) error {
	if strings.TrimSpace(value) == "" || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return invalidValue(field, "must be a non-empty relative path")
	}
	clean := filepath.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return invalidValue(field, "must remain within the working directory")
	}
	return nil
}

func parseDuration(field, value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, &ConfigurationError{
			Kind:  ConfigurationInvalidValueError,
			Field: field,
			Err:   errors.New("invalid duration"),
		}
	}
	if duration <= 0 {
		return 0, invalidValue(field, "must be empty or positive")
	}
	return duration, nil
}

func invalidValue(field, message string) error {
	return &ConfigurationError{
		Kind:  ConfigurationInvalidValueError,
		Field: field,
		Err:   fmt.Errorf("%s", message),
	}
}

func invalidCombination(field, message string) error {
	return &ConfigurationError{
		Kind:  ConfigurationInvalidCombinationError,
		Field: field,
		Err:   fmt.Errorf("%s", message),
	}
}
