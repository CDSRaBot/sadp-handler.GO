package network

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

func TestIssue62FixturesAreSecretFreeAndCorrelatable(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "62", "*.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("fixture count = %d, want 4", len(files))
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				XMLName xml.Name
			}
			if err := xml.Unmarshal(data, &envelope); err != nil {
				t.Fatalf("fixture XML is invalid: %v", err)
			}
			if envelope.XMLName.Local != "ProbeMatch" {
				t.Fatalf("fixture root = %q", envelope.XMLName.Local)
			}
			if _, err := logic.ParseMutationResponseIdentity(data); err != nil {
				t.Fatalf("fixture identity is invalid: %v", err)
			}
		})
	}
}
