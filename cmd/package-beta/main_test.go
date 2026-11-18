package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBetaVersionValidation(t *testing.T) {
	for _, tt := range []struct {
		version string
		valid   bool
	}{
		{"v1.2.3-beta", true}, {"v1.2.3-beta.1", true},
		{"1.2.3-beta", false}, {"v1.2.3", false}, {"v1.2.3-beta/../file", false},
	} {
		if got := betaVersion.MatchString(tt.version); got != tt.valid {
			t.Errorf("version %q accepted=%v", tt.version, got)
		}
	}
}

func TestBuildEnvironmentOverridesTarget(t *testing.T) {
	env := buildEnvironment([]string{"PATH=test", "GOOS=windows", "goarch=386", "CGO_ENABLED=1"}, platform{"linux", "amd64"})
	if got := strings.Join(env, "|"); got != "PATH=test|GOOS=linux|GOARCH=amd64|CGO_ENABLED=0" {
		t.Fatalf("unexpected environment: %s", got)
	}
}

func TestUntrackedBuildInputs(t *testing.T) {
	for _, name := range []string{"test.go", "source.S", "resource.syso", "module/go.mod", "go.sum"} {
		if !isBuildInput(name) {
			t.Errorf("%s not treated as a build input", name)
		}
	}
	for _, name := range []string{"libcrypto-1_1.dll", "notes.txt", ""} {
		if isBuildInput(name) {
			t.Errorf("%s incorrectly treated as a build input", name)
		}
	}
}

func TestCopyExclusiveNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	if err := os.WriteFile(src, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyExclusive(src, dst); err == nil {
		t.Fatal("existing destination was overwritten")
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "existing" {
		t.Fatalf("destination changed: %q, %v", data, err)
	}
}

func TestArchiveIncludesOnlySpecifiedFilesAndValidHash(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"binary": "synthetic binary", "README.md": "readme",
		filepath.Join("docs", "HELP.md"): "help", "secret.txt": "not packaged",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	archivePath := filepath.Join(root, "candidate.zip")
	if err := writeArchive(archivePath, root, filepath.Join(root, "binary"), "sadp-handler", []string{"README.md", "docs/HELP.md"}); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	want := map[string]string{"UNVERIFIED-BETA.txt": warning, "sadp-handler": "synthetic binary", "README.md": "readme", "docs/HELP.md": "help"}
	for _, entry := range archive.File {
		content, ok := want[entry.Name]
		if !ok {
			t.Fatalf("unexpected archive entry %q", entry.Name)
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		if string(data) != content {
			t.Errorf("incorrect contents of %s", entry.Name)
		}
		delete(want, entry.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing archive entries: %v", want)
	}
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(data)
	gotHash, err := fileHash(archivePath)
	if err != nil || gotHash != wantHash {
		t.Fatalf("hash=%s expected=%s error=%v", hex.EncodeToString(gotHash[:]), hex.EncodeToString(wantHash[:]), err)
	}
}

func TestArchiveRejectsUnsafePath(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "binary")
	if err := os.WriteFile(binary, []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := writeArchive(filepath.Join(root, "bad.zip"), root, binary, "sadp-handler", []string{filepath.Join("..", "secret")})
	if err == nil {
		t.Fatal("unsafe path was accepted")
	}
}
