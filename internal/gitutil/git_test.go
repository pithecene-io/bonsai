package gitutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justapithecus/bonsai/internal/gitutil"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Initialize a git repo
	if _, err := gitutil.Run(dir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := gitutil.Run(dir, "config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	if _, err := gitutil.Run(dir, "config", "user.name", "Test"); err != nil {
		t.Fatalf("git config name: %v", err)
	}

	// Create initial commit
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.Run(dir, "commit", "-m", "initial"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return dir
}

func TestIsInsideWorkTree(t *testing.T) {
	dir := setupTestRepo(t)
	if !gitutil.IsInsideWorkTree(dir) {
		t.Error("expected IsInsideWorkTree to return true")
	}
	if gitutil.IsInsideWorkTree(t.TempDir()) {
		t.Error("expected IsInsideWorkTree to return false for non-repo")
	}
}

func TestShowToplevel(t *testing.T) {
	dir := setupTestRepo(t)
	top, err := gitutil.ShowToplevel(dir)
	if err != nil {
		t.Fatalf("ShowToplevel: %v", err)
	}
	// Resolve symlinks for tmpdir comparison
	expected, _ := filepath.EvalSymlinks(dir)
	actual, _ := filepath.EvalSymlinks(top)
	if actual != expected {
		t.Errorf("ShowToplevel = %q, want %q", actual, expected)
	}
}

func TestCurrentBranch(t *testing.T) {
	dir := setupTestRepo(t)
	branch, err := gitutil.CurrentBranch(dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	// Default branch could be main or master depending on git config
	if branch != "main" && branch != "master" {
		t.Errorf("CurrentBranch = %q, want main or master", branch)
	}
}

func TestDiffNameOnly(t *testing.T) {
	dir := setupTestRepo(t)

	// Get current HEAD as base
	base, err := gitutil.RevParse(dir, "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	// Create a new file on a branch
	if _, err := gitutil.Run(dir, "checkout", "-b", "test-branch"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	f := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(f, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := gitutil.Run(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.Run(dir, "commit", "-m", "add new"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	files, err := gitutil.DiffNameOnly(dir, base)
	if err != nil {
		t.Fatalf("DiffNameOnly: %v", err)
	}
	if len(files) != 1 || files[0] != "new.txt" {
		t.Errorf("DiffNameOnly = %v, want [new.txt]", files)
	}
}

func TestUntrackedFiles(t *testing.T) {
	dir := setupTestRepo(t)

	// Create untracked file
	f := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(f, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	files, err := gitutil.UntrackedFiles(dir)
	if err != nil {
		t.Fatalf("UntrackedFiles: %v", err)
	}
	if len(files) != 1 || files[0] != "untracked.txt" {
		t.Errorf("UntrackedFiles = %v, want [untracked.txt]", files)
	}
}

func TestRefExists(t *testing.T) {
	dir := setupTestRepo(t)
	if !gitutil.RefExists(dir, "HEAD") {
		t.Error("expected HEAD to exist")
	}
	if gitutil.RefExists(dir, "nonexistent-ref") {
		t.Error("expected nonexistent-ref to not exist")
	}
}

func TestRunError(t *testing.T) {
	dir := t.TempDir() // not a git repo
	_, err := gitutil.Run(dir, "status")
	if err == nil {
		t.Error("expected error running git in non-repo")
	}
}
