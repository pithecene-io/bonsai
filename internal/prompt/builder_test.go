package prompt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/prompt"
)

func TestBuildInteractive_Architect(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	result, err := b.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModeArchitect,
		Role: "architect",
	})
	if err != nil {
		t.Fatalf("BuildInteractive: %v", err)
	}

	assertContains(t, result, "ARCHITECT mode")
	assertContains(t, result, "Repository root: /tmp/test-repo")
	assertContains(t, result, "CLAUDE.md")
	assertContains(t, result, "Do not write code unless explicitly asked.")
}

func TestBuildInteractive_Implementer(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	result, err := b.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModeImplementer,
		Role: "implementer",
	})
	if err != nil {
		t.Fatalf("BuildInteractive: %v", err)
	}

	assertContains(t, result, "IMPLEMENTER mode")
	// Implementer mode should NOT have "Do not write code" preamble
	if strings.Contains(result, "Do not write code unless explicitly asked.") {
		t.Error("implementer mode should not have architect preamble")
	}
}

func TestBuildInteractive_WithExtraContext(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	findings := "SKILL: repo-convention-enforcer | blocking: 2 | major: 0 | warning: 1"
	result, err := b.BuildInteractive(prompt.InteractiveOpts{
		Mode:         prompt.ModeImplementer,
		ExtraContext: findings,
	})
	if err != nil {
		t.Fatalf("BuildInteractive: %v", err)
	}

	assertContains(t, result, "Previous governance findings (fix these)")
	assertContains(t, result, findings)
}

func TestBuildInteractive_WithRepoFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Test AGENTS"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "ARCH_INDEX.md"), []byte("# Arch Index"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := assets.NewResolver(dir)
	b := prompt.NewBuilder(r, dir)

	result, err := b.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModeArchitect,
	})
	if err != nil {
		t.Fatalf("BuildInteractive: %v", err)
	}

	assertContains(t, result, "Repository context:")
	assertContains(t, result, "# Test AGENTS")
	assertContains(t, result, "Repository architecture index:")
	assertContains(t, result, "# Arch Index")
}

func TestBuildReview(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	result, err := b.BuildReview()
	if err != nil {
		t.Fatalf("BuildReview: %v", err)
	}

	assertContains(t, result, "REVIEWER mode")
	assertContains(t, result, "Review architecture:")
}

func TestBuildValidator(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	result, err := b.BuildValidator(prompt.ValidatorOpts{
		SkillBody:    "Check for convention violations.",
		OutputSchema: `{"type": "object"}`,
	})
	if err != nil {
		t.Fatalf("BuildValidator: %v", err)
	}

	assertContains(t, result, "VALIDATOR mode")
	assertContains(t, result, "Check for convention violations.")
	assertContains(t, result, `{"type": "object"}`)
	assertContains(t, result, "No markdown. No prose. No explanation. No code fences. JSON only.")
}

func TestBuildValidator_WithRepoClaude(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Repo Constitution"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := assets.NewResolver(dir)
	b := prompt.NewBuilder(r, dir)

	result, err := b.BuildValidator(prompt.ValidatorOpts{
		SkillBody:    "Test skill body",
		OutputSchema: "{}",
	})
	if err != nil {
		t.Fatalf("BuildValidator: %v", err)
	}

	assertContains(t, result, "Repo-local constitution (CLAUDE.md):")
	assertContains(t, result, "# Repo Constitution")
}

func TestBuildInteractive_InjectionOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# AGENTS-MARKER"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "ARCH_INDEX.md"), []byte("# ARCH-INDEX-MARKER"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := assets.NewResolver(dir)
	b := prompt.NewBuilder(r, dir)

	result, err := b.BuildInteractive(prompt.InteractiveOpts{
		Mode:         prompt.ModeImplementer,
		ExtraContext: "FINDINGS-MARKER",
	})
	if err != nil {
		t.Fatalf("BuildInteractive: %v", err)
	}

	// Required ordering: mode → CLAUDE.md → role → AGENTS.md → ARCH_INDEX → findings
	markers := []struct {
		label string
		text  string
	}{
		{"mode declaration", "IMPLEMENTER mode"},
		{"global CLAUDE.md", "CLAUDE.md — Constitution"},
		{"role definition", "precision implementation assistant"},
		{"AGENTS.md", "# AGENTS-MARKER"},
		{"ARCH_INDEX.md", "# ARCH-INDEX-MARKER"},
		{"extra context (findings)", "FINDINGS-MARKER"},
	}

	for i := 0; i < len(markers)-1; i++ {
		posA := strings.Index(result, markers[i].text)
		posB := strings.Index(result, markers[i+1].text)
		if posA < 0 {
			t.Errorf("missing %s (%q)", markers[i].label, markers[i].text)
			continue
		}
		if posB < 0 {
			t.Errorf("missing %s (%q)", markers[i+1].label, markers[i+1].text)
			continue
		}
		if posA >= posB {
			t.Errorf("injection order violation: %s (pos %d) must appear before %s (pos %d)",
				markers[i].label, posA, markers[i+1].label, posB)
		}
	}
}

func TestBuildValidator_InjectionOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# REPO-CLAUDE-MARKER"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# AGENTS-MARKER"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "ARCH_INDEX.md"), []byte("# ARCH-INDEX-MARKER"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := assets.NewResolver(dir)
	b := prompt.NewBuilder(r, dir)

	result, err := b.BuildValidator(prompt.ValidatorOpts{
		SkillBody:    "SKILL-BODY-MARKER",
		OutputSchema: "SCHEMA-MARKER",
	})
	if err != nil {
		t.Fatalf("BuildValidator: %v", err)
	}

	// Required ordering: mode → global CLAUDE.md → repo CLAUDE.md → AGENTS.md → ARCH_INDEX → skill → schema → JSON-only suffix
	markers := []struct {
		label string
		text  string
	}{
		{"mode declaration", "VALIDATOR mode"},
		{"global CLAUDE.md", "CLAUDE.md — Constitution"},
		{"repo CLAUDE.md", "# REPO-CLAUDE-MARKER"},
		{"AGENTS.md", "# AGENTS-MARKER"},
		{"ARCH_INDEX.md", "# ARCH-INDEX-MARKER"},
		{"skill body", "SKILL-BODY-MARKER"},
		{"output schema", "SCHEMA-MARKER"},
		{"JSON-only suffix", "No markdown. No prose."},
	}

	for i := 0; i < len(markers)-1; i++ {
		posA := strings.Index(result, markers[i].text)
		posB := strings.Index(result, markers[i+1].text)
		if posA < 0 {
			t.Errorf("missing %s (%q)", markers[i].label, markers[i].text)
			continue
		}
		if posB < 0 {
			t.Errorf("missing %s (%q)", markers[i+1].label, markers[i+1].text)
			continue
		}
		if posA >= posB {
			t.Errorf("injection order violation: %s (pos %d) must appear before %s (pos %d)",
				markers[i].label, posA, markers[i+1].label, posB)
		}
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected prompt to contain %q", needle)
	}
}
