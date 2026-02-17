package repo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justapithecus/bonsai/internal/gitutil"
	"github.com/justapithecus/bonsai/internal/repo"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if _, err := gitutil.Run(dir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := gitutil.Run(dir, "config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("git config: %v", err)
	}
	if _, err := gitutil.Run(dir, "config", "user.name", "Test"); err != nil {
		t.Fatalf("git config: %v", err)
	}

	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.Run(dir, "commit", "-m", "initial"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return dir
}

func TestDetect(t *testing.T) {
	dir := setupTestRepo(t)

	info, err := repo.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	resolved, _ := filepath.EvalSymlinks(dir)
	actualRoot, _ := filepath.EvalSymlinks(info.Root)
	if actualRoot != resolved {
		t.Errorf("Root = %q, want %q", actualRoot, resolved)
	}
	if info.Branch != "main" && info.Branch != "master" {
		t.Errorf("Branch = %q, want main or master", info.Branch)
	}
	if info.IsWorktree {
		t.Error("expected IsWorktree to be false for main checkout")
	}
}

func TestDetectNonRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := repo.Detect(dir)
	if err == nil {
		t.Error("expected error for non-repo directory")
	}
}

func TestDetectMergeBase(t *testing.T) {
	dir := setupTestRepo(t)

	// Create a branch
	if _, err := gitutil.Run(dir, "checkout", "-b", "feature"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	f := filepath.Join(dir, "feature.txt")
	if err := os.WriteFile(f, []byte("feature\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.Run(dir, "commit", "-m", "feature commit"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	mb := repo.DetectMergeBase(dir, nil)
	if mb == "" {
		t.Error("expected non-empty merge base")
	}
}

func TestDetectMergeBaseNoCandidates(t *testing.T) {
	dir := setupTestRepo(t)

	// Rename default branch to something not in candidates
	if _, err := gitutil.Run(dir, "branch", "-m", "unusual"); err != nil {
		t.Fatalf("branch rename: %v", err)
	}

	mb := repo.DetectMergeBase(dir, nil)
	if mb != "" {
		t.Errorf("expected empty merge base, got %q", mb)
	}
}

func TestTree(t *testing.T) {
	dir := setupTestRepo(t)

	// Add an untracked file
	f := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(f, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tree, err := repo.Tree(dir)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}

	// Should have README.md (tracked) and untracked.txt
	want := map[string]bool{"README.md": false, "untracked.txt": false}
	for _, f := range tree {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Errorf("Tree missing %q", k)
		}
	}
}

func TestTreeWithScope(t *testing.T) {
	dir := setupTestRepo(t)

	// Create files in different directories
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "README.md"), []byte("# Docs\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}

	tree, err := repo.TreeWithScope(dir, "src/")
	if err != nil {
		t.Fatalf("TreeWithScope: %v", err)
	}

	if len(tree) != 1 || tree[0] != "src/main.go" {
		t.Errorf("TreeWithScope(src/) = %v, want [src/main.go]", tree)
	}
}
