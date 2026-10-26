package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/config"
)

func TestComposeBuildsTypedRuntimeConfig(t *testing.T) {
	settings := config.Defaults()
	settings.Localization.Directory = "locale-data"
	baseDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(baseDir, "locale-data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "locale-data", "ru.json"), []byte(`{"ui.help":"help"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := Compose(settings, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.UI.Mode != "yes" || runtime.Network.Group.String() != "239.255.255.250:37020" {
		t.Fatalf("runtime = %#v", runtime)
	}
	if runtime.Discovery.MaxSADPDatagramSize != settings.Network.MaxSADPDatagramSize {
		t.Fatalf("datagram size was not preserved")
	}
	if runtime.Protocol != "sadp_first" {
		t.Fatalf("protocol strategy = %q, want sadp_first", runtime.Protocol)
	}
	if _, err := fs.ReadFile(runtime.Localization.FS, "locales/ru.json"); err != nil {
		t.Fatalf("configured localization directory was not mapped: %v", err)
	}
}

func TestComposeRejectsEscapingLocalizationDirectory(t *testing.T) {
	settings := config.Defaults()
	settings.Localization.Directory = filepath.Join("..", "outside")
	if _, err := Compose(settings, t.TempDir()); err == nil {
		t.Fatal("escaping localization path was accepted")
	}
}

func TestBuildDependenciesMapsRuntimePolicy(t *testing.T) {
	runtime, err := Compose(config.Defaults(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dependencies := BuildDependencies(runtime, ServiceSet{})
	if dependencies.Services.UI.Config != runtime.UI {
		t.Fatalf("UI config = %#v, want %#v", dependencies.Services.UI.Config, runtime.UI)
	}
	if dependencies.Services.UI.DiscoveryPolicy.CycleTimeout != runtime.Discovery.CycleTimeout {
		t.Fatalf("discovery policy was not mapped")
	}
}
