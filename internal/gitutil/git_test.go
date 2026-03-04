package gitutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pithecene-io/bonsai/internal/gitutil"
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

func TestCheckInsideWorkTree(t *testing.T) {
	t.Run("inside repo returns true with no error", func(t *testing.T) {
		dir := setupTestRepo(t)
		inside, err := gitutil.CheckInsideWorkTree(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !inside {
			t.Error("expected true for git repo")
		}
	})

	t.Run("non-repo returns false with no error", func(t *testing.T) {
		dir := t.TempDir()
		inside, err := gitutil.CheckInsideWorkTree(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inside {
			t.Error("expected false for non-repo")
		}
	})

	t.Run("nonexistent dir returns false with error", func(t *testing.T) {
		inside, err := gitutil.CheckInsideWorkTree("/nonexistent/path/that/does/not/exist")
		if inside {
			t.Error("expected false for nonexistent path")
		}
		// Error may or may not be returned depending on git version,
		// but inside must be false.
		_ = err
	})
}

func TestCreateAndRemoveWorktree(t *testing.T) {
	dir := setupTestRepo(t)
	wtPath := filepath.Join(t.TempDir(), "test-worktree")
	branch := "test/worktree-branch"

	// Create worktree
	if err := gitutil.CreateWorktree(dir, wtPath, branch); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Verify worktree exists and branch was created
	if !gitutil.IsInsideWorkTree(wtPath) {
		t.Error("worktree directory is not a git work tree")
	}
	if !gitutil.RefExists(dir, branch) {
		t.Error("branch was not created")
	}

	// Remove worktree — branch should still exist
	if err := gitutil.RemoveWorktree(dir, wtPath); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if gitutil.IsInsideWorkTree(wtPath) {
		t.Error("worktree directory should not be a work tree after removal")
	}
	if !gitutil.RefExists(dir, branch) {
		t.Error("RemoveWorktree should not delete the branch")
	}

	// Delete branch
	if err := gitutil.DeleteBranch(dir, branch); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	if gitutil.RefExists(dir, branch) {
		t.Error("branch should not exist after DeleteBranch")
	}
}

func TestIsDirty(t *testing.T) {
	t.Run("clean repo", func(t *testing.T) {
		dir := setupTestRepo(t)
		dirty, err := gitutil.IsDirty(dir)
		if err != nil {
			t.Fatalf("IsDirty: %v", err)
		}
		if dirty {
			t.Error("expected clean repo to not be dirty")
		}
	})

	t.Run("dirty repo", func(t *testing.T) {
		dir := setupTestRepo(t)
		f := filepath.Join(dir, "dirty.txt")
		if err := os.WriteFile(f, []byte("dirty\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		dirty, err := gitutil.IsDirty(dir)
		if err != nil {
			t.Fatalf("IsDirty: %v", err)
		}
		if !dirty {
			t.Error("expected dirty repo to be dirty")
		}
	})
}
