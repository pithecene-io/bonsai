package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pithecene-io/bonsai/internal/skill"
)

func TestBuildSyntheticUntrackedDiff(t *testing.T) {
	// Create a temp directory with test files
	dir := t.TempDir()

	// Create a regular file
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create an executable file
	if err := os.WriteFile(filepath.Join(dir, "script.sh"), []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := skill.BuildSyntheticUntrackedDiff(dir, []string{"new.go", "script.sh"})

	// Should contain diff headers for both files
	if !strings.Contains(result, "diff --git a/new.go b/new.go") {
		t.Error("missing diff header for new.go")
	}
	if !strings.Contains(result, "new file mode 100644") {
		t.Error("missing file mode 100644 for new.go")
	}
	if !strings.Contains(result, "diff --git a/script.sh b/script.sh") {
		t.Error("missing diff header for script.sh")
	}
	if !strings.Contains(result, "new file mode 100755") {
		t.Error("missing file mode 100755 for script.sh")
	}

	// Should contain content lines prefixed with +
	if !strings.Contains(result, "+line1") {
		t.Error("missing +line1 in diff body")
	}

	// Should have --- /dev/null header
	if !strings.Contains(result, "--- /dev/null") {
		t.Error("missing --- /dev/null header")
	}
}

func TestBuildSyntheticUntrackedDiff_NonexistentFile(t *testing.T) {
	dir := t.TempDir()

	// Should silently skip nonexistent files
	result := skill.BuildSyntheticUntrackedDiff(dir, []string{"doesnotexist.go"})
	if result != "" {
		t.Errorf("expected empty result for nonexistent file, got: %q", result)
	}
}

func TestBuildSyntheticUntrackedDiff_EmptyList(t *testing.T) {
	dir := t.TempDir()

	result := skill.BuildSyntheticUntrackedDiff(dir, nil)
	if result != "" {
		t.Errorf("expected empty result for nil file list, got: %q", result)
	}
}

func TestBuildSyntheticUntrackedDiff_Directory(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory — should be skipped (not regular file)
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	result := skill.BuildSyntheticUntrackedDiff(dir, []string{"subdir"})
	if result != "" {
		t.Errorf("expected empty result for directory, got: %q", result)
	}
}
