package doccheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckAcceptsCompleteNormativeFixture(t *testing.T) {
	root := writeFixture(t, true, true, true)

	if violations := Check(root); len(violations) != 0 {
		t.Fatalf("Check() returned violations: %v", violations)
	}
}

func TestCheckRejectsMissingRussianCounterpart(t *testing.T) {
	root := writeFixture(t, true, true, true)
	if err := os.Remove(filepath.Join(root, "docs", "ru", "spec", "modules", "network.md")); err != nil {
		t.Fatal(err)
	}

	assertViolation(t, Check(root), "missing Russian normative counterpart")
}

func TestCheckRejectsBrokenRelativeLink(t *testing.T) {
	root := writeFixture(t, true, true, true)
	path := filepath.Join(root, "docs", "spec", "modules", "network.md")
	appendFile(t, path, "\n[broken](../missing.md)\n")

	assertViolation(t, Check(root), "broken relative link")
}

func TestCheckRejectsStaleStatusClaim(t *testing.T) {
	root := writeFixture(t, true, true, true)
	path := filepath.Join(root, "docs", "spec", "SYSTEM.md")
	appendFile(t, path, "\nThe Go implementation has not started.\n")

	assertViolation(t, Check(root), "stale implementation-status claim")
}

func TestCheckRejectsIncompleteM1Contract(t *testing.T) {
	root := writeFixture(t, true, true, false)

	assertViolation(t, Check(root), `M1 contract is missing "MutationTransport"`)
}

func writeFixture(t *testing.T, russian, links, completeM1 bool) string {
	t.Helper()
	root := t.TempDir()
	en := filepath.Join(root, "docs", "spec")
	ru := filepath.Join(root, "docs", "ru", "spec")
	for _, dir := range []string{
		filepath.Join(en, "modules"),
		filepath.Join(ru, "modules"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	system := "# System\n"
	m1 := "MutationTarget MutationExchange MutationTransport sender ownership response_too_large\n"
	if !completeM1 {
		m1 = "MutationTarget\n"
	}
	if links {
		system += "[M1](./modules/network.md)\n"
	}
	writeFile(t, filepath.Join(en, "SYSTEM.md"), system)
	writeFile(t, filepath.Join(en, "modules", "network.md"), m1)
	if russian {
		writeFile(t, filepath.Join(ru, "SYSTEM.md"), "# Система\n")
		writeFile(t, filepath.Join(ru, "modules", "network.md"), m1)
	}
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func assertViolation(t *testing.T, violations []Violation, want string) {
	t.Helper()
	for _, violation := range violations {
		if strings.Contains(violation.Message, want) {
			return
		}
	}
	t.Fatalf("expected violation containing %q, got %v", want, violations)
}
