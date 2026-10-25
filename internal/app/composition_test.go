package app

import (
	"path/filepath"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/config"
)

func TestComposeBuildsTypedRuntimeConfig(t *testing.T) {
	settings := config.Defaults()
	settings.Localization.Directory = "locale-data"
	runtime, err := Compose(settings, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.UI.Mode != "yes" || runtime.Network.Group.String() != "239.255.255.250:37020" {
		t.Fatalf("runtime = %#v", runtime)
	}
	if runtime.Discovery.MaxSADPDatagramSize != settings.Network.MaxSADPDatagramSize {
		t.Fatalf("datagram size was not preserved")
	}
}

func TestComposeRejectsEscapingLocalizationDirectory(t *testing.T) {
	settings := config.Defaults()
	settings.Localization.Directory = filepath.Join("..", "outside")
	if _, err := Compose(settings, t.TempDir()); err == nil {
		t.Fatal("escaping localization path was accepted")
	}
}
