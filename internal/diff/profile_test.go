package diff_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/diff"
	"github.com/pithecene-io/bonsai/internal/gitutil"
)

// setupTestRepo creates a temporary git repo with an initial commit and
// returns the repo root and the base commit SHA.
func setupTestRepo(t *testing.T) (repoRoot, baseSHA string) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		if _, err := gitutil.Run(dir, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	run("add", ".")
	run("commit", "-m", "initial")

	base, err := gitutil.RevParse(dir, "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	// Create a feature branch so we diff against the initial commit.
	run("checkout", "-b", "feature")

	return dir, base
}

func TestComputeProfile_TrackedChanges(t *testing.T) {
	dir, base := setupTestRepo(t)
	cfg := config.Default()

	// Add a tracked file with known content.
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.Run(dir, "commit", "-m", "add main.go"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	p, err := diff.ComputeProfile(dir, base, cfg)
	if err != nil {
		t.Fatalf("ComputeProfile: %v", err)
	}

	if p.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1", p.FilesChanged)
	}
	if p.NewFiles != 1 {
		t.Errorf("NewFiles = %d, want 1", p.NewFiles)
	}
	if p.LinesAdded < 1 {
		t.Errorf("LinesAdded = %d, want >= 1", p.LinesAdded)
	}
	if p.DiffLines < 1 {
		t.Errorf("DiffLines = %d, want >= 1", p.DiffLines)
	}
}

func TestComputeProfile_UntrackedOnly(t *testing.T) {
	dir, base := setupTestRepo(t)
	cfg := config.Default()

	// Create untracked files (not staged, not committed).
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	p, err := diff.ComputeProfile(dir, base, cfg)
	if err != nil {
		t.Fatalf("ComputeProfile: %v", err)
	}

	// Exactly 1 untracked file must appear in both FilesChanged and NewFiles.
	if p.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1", p.FilesChanged)
	}
	if p.NewFiles != 1 {
		t.Errorf("NewFiles = %d, want 1", p.NewFiles)
	}
	// DiffLines should be 0 — git diff does not include untracked files.
	if p.DiffLines != 0 {
		t.Errorf("DiffLines = %d, want 0 (untracked files have no diff output)", p.DiffLines)
	}
}

func TestComputeProfile_MixedTrackedAndUntracked(t *testing.T) {
	dir, base := setupTestRepo(t)
	cfg := config.Default()

	// Tracked change: modify existing file.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Updated\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.Run(dir, "commit", "-m", "update readme"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	// Untracked file.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("notes\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	p, err := diff.ComputeProfile(dir, base, cfg)
	if err != nil {
		t.Fatalf("ComputeProfile: %v", err)
	}

	// 1 tracked changed file + 1 untracked = 2 files changed.
	if p.FilesChanged != 2 {
		t.Errorf("FilesChanged = %d, want 2", p.FilesChanged)
	}
	// README.md is modified (M status), notes.txt is new (A via untracked synthetic entry).
	// Exactly 1 new file.
	if p.NewFiles != 1 {
		t.Errorf("NewFiles = %d, want 1 (only notes.txt is new; README.md is modified)", p.NewFiles)
	}
}

func TestComputeProfile_PublicSurfaceDetection(t *testing.T) {
	dir, base := setupTestRepo(t)
	cfg := config.Default()

	// Create a file under api/ (matches default PublicSurfaceGlobs).
	if err := os.MkdirAll(filepath.Join(dir, "api", "v1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api", "v1", "handler.go"), []byte("package v1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.Run(dir, "commit", "-m", "add api handler"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	p, err := diff.ComputeProfile(dir, base, cfg)
	if err != nil {
		t.Fatalf("ComputeProfile: %v", err)
	}

	if len(p.PublicSurfacePaths) != 1 {
		t.Fatalf("PublicSurfacePaths len = %d, want 1; got %v", len(p.PublicSurfacePaths), p.PublicSurfacePaths)
	}
	if p.PublicSurfacePaths[0] != "api/v1/handler.go" {
		t.Errorf("PublicSurfacePaths[0] = %q, want \"api/v1/handler.go\"", p.PublicSurfacePaths[0])
	}
}

func TestComputeProfile_StructuralDetection(t *testing.T) {
	dir, base := setupTestRepo(t)
	cfg := config.Default()

	// Create a file under orchestrator/ (matches default StructuralPatterns).
	if err := os.MkdirAll(filepath.Join(dir, "internal", "orchestrator"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "orchestrator", "run.go"), []byte("package orchestrator\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.Run(dir, "commit", "-m", "add orchestrator"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	p, err := diff.ComputeProfile(dir, base, cfg)
	if err != nil {
		t.Fatalf("ComputeProfile: %v", err)
	}

	if !p.HasStructural {
		t.Error("HasStructural = false, want true (orchestrator/ matches structural pattern)")
	}
}

func TestComputeProfile_TopLevelDirs(t *testing.T) {
	dir, base := setupTestRepo(t)
	cfg := config.Default()

	// Create files in two different top-level directories.
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "app.go"), []byte("package src\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "guide.md"), []byte("# Guide\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.Run(dir, "commit", "-m", "add src and docs"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	p, err := diff.ComputeProfile(dir, base, cfg)
	if err != nil {
		t.Fatalf("ComputeProfile: %v", err)
	}

	if len(p.TopLevelDirs) != 2 {
		t.Fatalf("TopLevelDirs len = %d, want 2; got %v", len(p.TopLevelDirs), p.TopLevelDirs)
	}
	// Verify both expected dirs are present (order is non-deterministic from map iteration).
	dirSet := make(map[string]bool, len(p.TopLevelDirs))
	for _, d := range p.TopLevelDirs {
		dirSet[d] = true
	}
	if !dirSet["src"] {
		t.Errorf("TopLevelDirs missing \"src\"; got %v", p.TopLevelDirs)
	}
	if !dirSet["docs"] {
		t.Errorf("TopLevelDirs missing \"docs\"; got %v", p.TopLevelDirs)
	}
}
