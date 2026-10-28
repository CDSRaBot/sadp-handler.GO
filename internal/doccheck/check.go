// Package doccheck validates the repository's normative documentation contracts.
package doccheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Violation describes one documentation consistency failure.
type Violation struct {
	Path    string
	Message string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s: %s", v.Path, v.Message)
}

var markdownLink = regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)`)

var staleClaims = []string{
	"the go implementation has not started",
	"реализация go ещё не начата",
	"до реализации окончательно определить пути пакетов",
}

var m1Markers = []string{
	"MutationTarget",
	"MutationExchange",
	"MutationTransport",
	"sender",
	"ownership",
	"response_too_large",
}

// Check validates normative English/Russian specification pairs and links.
func Check(root string) []Violation {
	var violations []Violation
	enRoot := filepath.Join(root, "docs", "spec")
	ruRoot := filepath.Join(root, "docs", "ru", "spec")

	enFiles := markdownFiles(enRoot)
	ruFiles := markdownFiles(ruRoot)
	ruSet := make(map[string]struct{}, len(ruFiles))
	for _, path := range ruFiles {
		rel, err := filepath.Rel(ruRoot, path)
		if err == nil {
			ruSet[filepath.Clean(rel)] = struct{}{}
		}
	}

	for _, path := range enFiles {
		rel, err := filepath.Rel(enRoot, path)
		if err != nil {
			continue
		}
		rel = filepath.Clean(rel)
		if isReport(rel) {
			continue
		}
		if _, ok := ruSet[rel]; !ok {
			violations = append(violations, Violation{
				Path:    filepath.ToSlash(filepath.Join("docs/spec", rel)),
				Message: "missing Russian normative counterpart",
			})
		}
	}

	violations = append(violations, checkLinks(root, enFiles)...)
	violations = append(violations, checkLinks(root, ruFiles)...)
	violations = append(violations, checkClaims(root, enFiles, ruFiles)...)
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Path == violations[j].Path {
			return violations[i].Message < violations[j].Message
		}
		return violations[i].Path < violations[j].Path
	})
	return violations
}

func markdownFiles(root string) []string {
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func isReport(rel string) bool {
	return strings.HasPrefix(filepath.ToSlash(rel), "reports/")
}

func checkLinks(root string, files []string) []Violation {
	var violations []Violation
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			violations = append(violations, Violation{Path: relativePath(root, path), Message: "cannot read file: " + err.Error()})
			continue
		}
		for _, match := range markdownLink.FindAllStringSubmatch(string(data), -1) {
			ref := strings.SplitN(match[1], "#", 2)[0]
			if ref == "" || strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "mailto:") {
				continue
			}
			target := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(ref)))
			if _, err := os.Stat(target); err != nil {
				violations = append(violations, Violation{
					Path:    relativePath(root, path),
					Message: fmt.Sprintf("broken relative link %q", ref),
				})
			}
		}
	}
	return violations
}

func checkClaims(root string, files ...[]string) []Violation {
	var violations []Violation
	for _, group := range files {
		for _, path := range group {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			lower := strings.ToLower(string(data))
			for _, claim := range staleClaims {
				if strings.Contains(lower, claim) {
					violations = append(violations, Violation{
						Path:    relativePath(root, path),
						Message: fmt.Sprintf("stale implementation-status claim %q", claim),
					})
				}
			}
		}
	}

	for _, rel := range []string{"docs/spec/modules/network.md", "docs/ru/spec/modules/network.md"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			violations = append(violations, Violation{Path: rel, Message: "cannot read M1 specification: " + err.Error()})
			continue
		}
		text := string(data)
		for _, marker := range m1Markers {
			if !strings.Contains(text, marker) {
				violations = append(violations, Violation{Path: rel, Message: fmt.Sprintf("M1 contract is missing %q", marker)})
			}
		}
	}
	return violations
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
