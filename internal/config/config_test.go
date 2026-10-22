package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsMatchDocumentedBaseline(t *testing.T) {
	settings := Defaults()
	if settings.UI.Mode != "yes" || settings.UI.Format != "text" || settings.UI.Language != "en" || settings.UI.Once {
		t.Fatalf("unexpected UI defaults: %#v", settings.UI)
	}
	if settings.Localization.Directory != "locales" {
		t.Fatalf("localization directory = %q, want locales", settings.Localization.Directory)
	}
	if settings.Network.MulticastGroup != "239.255.255.250" ||
		settings.Network.MulticastPort != 37020 ||
		settings.Network.MulticastTTL != 64 ||
		settings.Network.MaxSADPDatagramSize != 1500 {
		t.Fatalf("unexpected network defaults: %#v", settings.Network)
	}
	if settings.Discovery.ProbeInterval != "" || settings.Discovery.CycleTimeout != "5s" || settings.Discovery.RepeatCount != 1 {
		t.Fatalf("unexpected discovery defaults: %#v", settings.Discovery)
	}
	if settings.Logging.Level != "info" {
		t.Fatalf("logging level = %q, want info", settings.Logging.Level)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	settings, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if settings != Defaults() {
		t.Fatalf("settings = %#v, want defaults %#v", settings, Defaults())
	}
}

func TestLoadTemplate(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "settings.json"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	settings, err := Load(path)
	if err != nil {
		t.Fatalf("Load(template) returned error: %v", err)
	}
	if settings.Network.MulticastGroup != "239.255.255.250" || settings.Discovery.CycleTimeout != "5s" {
		t.Fatalf("template settings were not loaded: %#v", settings)
	}
}

func TestLoadRejectsMalformedTrailingAndUnknownJSON(t *testing.T) {
	testCases := []struct {
		name string
		data string
		kind ConfigurationErrorKind
	}{
		{name: "malformed", data: `{"_comments":`, kind: ConfigurationDecodeError},
		{name: "trailing", data: validDocument() + ` {}`, kind: ConfigurationDecodeError},
		{name: "unknown setting", data: withDocumentChange(func(document map[string]any) {
			document["unknown"] = true
		}), kind: ConfigurationDecodeError},
		{name: "unknown comment", data: withDocumentChange(func(document map[string]any) {
			document["_comments"].(map[string]any)["unknown"] = "not allowed"
		}), kind: ConfigurationUnknownKeyError},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := loadData(t, testCase.data)
			assertConfigurationError(t, err, testCase.kind)
		})
	}
}

func TestLoadRequiresEveryComment(t *testing.T) {
	data := withDocumentChange(func(document map[string]any) {
		delete(document["_comments"].(map[string]any), "logging.level")
	})
	err := loadData(t, data)
	assertConfigurationError(t, err, ConfigurationMissingCommentError)
}

func TestLoadValidatesDocumentedBoundaries(t *testing.T) {
	testCases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"port below lower bound", func(document map[string]any) {
			document["network"].(map[string]any)["multicast_port"] = 0
		}},
		{"port above upper bound", func(document map[string]any) {
			document["network"].(map[string]any)["multicast_port"] = 65536
		}},
		{"TTL below lower bound", func(document map[string]any) {
			document["network"].(map[string]any)["multicast_ttl"] = 0
		}},
		{"datagram size above upper bound", func(document map[string]any) {
			document["network"].(map[string]any)["max_sadp_datagram_size"] = 65508
		}},
		{"invalid multicast address", func(document map[string]any) {
			document["network"].(map[string]any)["multicast_group"] = "192.0.2.1"
		}},
		{"absolute locale path", func(document map[string]any) {
			document["localization"].(map[string]any)["directory"] = filepath.VolumeName("C:\\locales") + "\\locales"
		}},
		{"invalid duration", func(document map[string]any) {
			document["discovery"].(map[string]any)["cycle_timeout"] = "not-a-duration"
		}},
		{"invalid logging level", func(document map[string]any) {
			document["logging"].(map[string]any)["level"] = "trace"
		}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := loadData(t, withDocumentChange(testCase.change))
			assertConfigurationError(t, err, ConfigurationInvalidValueError)
		})
	}
}

func TestLoadValidatesDiscoveryCombinations(t *testing.T) {
	testCases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"continuous with bounded settings", func(document map[string]any) {
			discovery := document["discovery"].(map[string]any)
			discovery["probe_interval"] = "1m"
		}},
		{"bounded without timeout", func(document map[string]any) {
			discovery := document["discovery"].(map[string]any)
			discovery["cycle_timeout"] = ""
		}},
		{"repeat count too high", func(document map[string]any) {
			document["discovery"].(map[string]any)["repeat_count"] = 101
		}},
		{"once requires CLI", func(document map[string]any) {
			document["ui"].(map[string]any)["once"] = true
		}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := loadData(t, withDocumentChange(testCase.change))
			assertConfigurationError(t, err, ConfigurationInvalidCombinationError)
		})
	}
}

func TestLoadErrorsDoNotExposeDocumentSecrets(t *testing.T) {
	const secret = "synthetic-secret-not-for-logs"
	data := withDocumentChange(func(document map[string]any) {
		document["logging"].(map[string]any)["level"] = secret
	})
	err := loadData(t, data)
	if err == nil || !strings.Contains(err.Error(), "logging.level") {
		t.Fatalf("error = %v, want classified field error", err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "<") {
		t.Fatalf("error disclosed secret or document: %v", err)
	}
	var configurationErr *ConfigurationError
	if !errors.As(err, &configurationErr) {
		t.Fatalf("error = %T, want *ConfigurationError", err)
	}
}

func loadData(t *testing.T, data string) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	_, err := Load(path)
	return err
}

func assertConfigurationError(t *testing.T, err error, want ConfigurationErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("Load returned nil error, want %q", want)
	}
	var configurationErr *ConfigurationError
	if !errors.As(err, &configurationErr) {
		t.Fatalf("error = %T, want *ConfigurationError", err)
	}
	if configurationErr.Kind != want {
		t.Fatalf("error kind = %q, want %q", configurationErr.Kind, want)
	}
}

func validDocument() string {
	return withDocumentChange(nil)
}

func withDocumentChange(change func(map[string]any)) string {
	document := map[string]any{
		"_comments": map[string]any{
			"ui.mode":                        "mode",
			"ui.format":                      "format",
			"ui.language":                    "language",
			"ui.once":                        "once",
			"localization.directory":         "directory",
			"network.multicast_group":        "group",
			"network.multicast_port":         "port",
			"network.multicast_ttl":          "ttl",
			"network.max_sadp_datagram_size": "size",
			"discovery.probe_interval":       "probe interval",
			"discovery.cycle_timeout":        "cycle timeout",
			"discovery.repeat_count":         "repeat count",
			"logging.level":                  "level",
		},
		"ui": map[string]any{
			"mode": "yes", "format": "text", "language": "en", "once": false,
		},
		"localization": map[string]any{"directory": "locales"},
		"network": map[string]any{
			"multicast_group": "239.255.255.250", "multicast_port": 37020,
			"multicast_ttl": 64, "max_sadp_datagram_size": 1500,
		},
		"discovery": map[string]any{
			"probe_interval": "", "cycle_timeout": "5s", "repeat_count": 1,
		},
		"logging": map[string]any{"level": "info"},
	}
	if change != nil {
		change(document)
	}
	data, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	return string(data)
}
