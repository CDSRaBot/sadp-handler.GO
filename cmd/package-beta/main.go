// Command package-beta creates local, explicitly unverified beta candidates.
// It does not publish releases or certify device or operating-system support.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var betaVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+-beta(?:\.[0-9]+)?$`)

const warning = `UNVERIFIED BETA CANDIDATE

This package is unsigned and has not passed the project's real-device and
native-platform acceptance gates. It is not a published beta release.
Device activation and network configuration are not available in the
executable. See docs/BETA_PLAN.md for current scope and release requirements.
`

type platform struct {
	os   string
	arch string
}

var platforms = []platform{{"windows", "amd64"}, {"linux", "amd64"}, {"darwin", "amd64"}}

func main() {
	version := flag.String("version", "", "beta version, for example v0.1.0-beta.1")
	output := flag.String("output", "", "new output directory (must not already exist)")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	if err := packageBeta(*version, *output); err != nil {
		fmt.Fprintln(os.Stderr, "beta packaging failed:", err)
		os.Exit(1)
	}
}

func packageBeta(version, output string) (err error) {
	if !betaVersion.MatchString(version) || output == "" {
		return errors.New("a beta version and output directory are required")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(output); err == nil {
		return errors.New("output path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	status, err := commandOutput(root, "git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(status)) != 0 {
		return errors.New("tracked worktree changes must be committed before packaging")
	}
	untracked, err := commandOutput(root, "git", "ls-files", "-z", "--others")
	if err != nil {
		return err
	}
	for _, entry := range bytes.Split(untracked, []byte{0}) {
		if isBuildInput(string(entry)) {
			return errors.New("untracked build inputs must be removed or committed before packaging")
		}
	}
	tracked, err := commandOutput(root, "git", "ls-files", "-z", "--", "docs")
	if err != nil {
		return err
	}
	files := []string{"README.md", "LICENSE", "settings.json"}
	for _, entry := range bytes.Split(tracked, []byte{0}) {
		if len(entry) != 0 {
			files = append(files, string(entry))
		}
	}
	if len(files) == 3 {
		return errors.New("tracked documentation is missing")
	}
	sort.Strings(files)
	stage, err := os.MkdirTemp(filepath.Dir(output), ".beta-package-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(stage)) }()
	var checksums strings.Builder
	for _, target := range platforms {
		name := "sadp-handler-" + version + "-" + target.os + "-" + target.arch + ".zip"
		binaryName := "sadp-handler"
		if target.os == "windows" {
			binaryName += ".exe"
		}
		binaryPath := filepath.Join(stage, binaryName)
		build := exec.Command("go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-ldflags", "-X=main.version="+version, "-o", binaryPath, "./cmd/sadp-handler")
		build.Dir = root
		build.Env = buildEnvironment(os.Environ(), target)
		buildOutput, err := build.CombinedOutput()
		if err != nil {
			return fmt.Errorf("build %s/%s: %w: %s", target.os, target.arch, err, strings.TrimSpace(string(buildOutput)))
		}
		archive := filepath.Join(stage, name)
		if err := writeArchive(archive, root, binaryPath, binaryName, files); err != nil {
			return fmt.Errorf("package %s/%s: %w", target.os, target.arch, err)
		}
		if err := os.Remove(binaryPath); err != nil {
			return err
		}
		sum, err := fileHash(archive)
		if err != nil {
			return err
		}
		fmt.Fprintf(&checksums, "%x  %s\n", sum, name)
	}
	if err := os.WriteFile(filepath.Join(stage, "SHA256SUMS"), []byte(checksums.String()), 0o644); err != nil {
		return err
	}
	if err := os.Mkdir(output, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	for _, target := range platforms {
		name := "sadp-handler-" + version + "-" + target.os + "-" + target.arch + ".zip"
		if err := copyExclusive(filepath.Join(stage, name), filepath.Join(output, name)); err != nil {
			return fmt.Errorf("incomplete output directory %q: %w", output, err)
		}
	}
	if err := copyExclusive(filepath.Join(stage, "SHA256SUMS"), filepath.Join(output, "SHA256SUMS")); err != nil {
		return fmt.Errorf("incomplete output directory %q: %w", output, err)
	}
	return nil
}

func isBuildInput(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go", ".syso", ".s", ".c", ".h", ".cc", ".cpp":
		return true
	}
	base := strings.ToLower(filepath.Base(name))
	return base == "go.mod" || base == "go.sum"
}

func copyExclusive(source, destination string) (err error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, input.Close()) }()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, output.Close()) }()
	_, err = io.Copy(output, input)
	return err
}

func commandOutput(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	return output, nil
}

func buildEnvironment(env []string, target platform) []string {
	result := make([]string, 0, len(env)+3)
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "GOOS") && !strings.EqualFold(key, "GOARCH") && !strings.EqualFold(key, "CGO_ENABLED") {
			result = append(result, entry)
		}
	}
	return append(result, "GOOS="+target.os, "GOARCH="+target.arch, "CGO_ENABLED=0")
}

func writeArchive(destination, root, binaryPath, binaryName string, files []string) (err error) {
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, output.Close()) }()
	archive := zip.NewWriter(output)
	defer func() { err = errors.Join(err, archive.Close()) }()
	if err := writeEntry(archive, "UNVERIFIED-BETA.txt", strings.NewReader(warning), 0o644); err != nil {
		return err
	}
	if err := writeFileEntry(archive, binaryPath, binaryName, 0o755); err != nil {
		return err
	}
	for _, name := range files {
		if filepath.IsAbs(name) || !filepath.IsLocal(name) {
			return errors.New("unsafe documentation path")
		}
		if err := writeFileEntry(archive, filepath.Join(root, filepath.FromSlash(name)), filepath.ToSlash(name), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeFileEntry(archive *zip.Writer, path, name string, mode os.FileMode) (err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("package source %q is not a regular file", name)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	return writeEntry(archive, name, file, mode)
}

func writeEntry(archive *zip.Writer, name string, source io.Reader, mode os.FileMode) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, source)
	return err
}

func fileHash(path string) (sum [sha256.Size]byte, err error) {
	file, err := os.Open(path)
	if err != nil {
		return sum, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return sum, err
	}
	copy(sum[:], hash.Sum(nil))
	return sum, nil
}
