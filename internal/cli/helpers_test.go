package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pithecene-io/bonsai/internal/gitutil"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := gitutil.Run(dir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := gitutil.Run(dir, "config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	if _, err := gitutil.Run(dir, "config", "user.name", "Test"); err != nil {
		t.Fatalf("git config name: %v", err)
	}
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

func TestEnsureFeatureBranch_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	wt, err := ensureFeatureBranch(dir, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.RepoRoot != dir {
		t.Errorf("RepoRoot = %q, want %q", wt.RepoRoot, dir)
	}
	if wt.IsWorktree {
		t.Error("should not create worktree for non-git dir")
	}
}

func TestEnsureFeatureBranch_AlreadyOnFeatureBranch(t *testing.T) {
	dir := setupGitRepo(t)
	if _, err := gitutil.Run(dir, "checkout", "-b", "feature/test"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	wt, err := ensureFeatureBranch(dir, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.RepoRoot != dir {
		t.Errorf("RepoRoot = %q, want %q", wt.RepoRoot, dir)
	}
	if wt.IsWorktree {
		t.Error("should not create worktree when on feature branch")
	}
}

func TestEnsureFeatureBranch_CreatesWorktreeOnMain(t *testing.T) {
	dir := setupGitRepo(t)

	// Save and restore CWD — ensureFeatureBranch calls os.Chdir.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	wt, err := ensureFeatureBranch(dir, "implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.IsWorktree {
		t.Fatal("expected worktree to be created on main")
	}
	if wt.RepoRoot == dir {
		t.Error("RepoRoot should differ from original dir")
	}

	// Verify CWD was changed to the worktree.
	cwd, _ := os.Getwd()
	wtResolved, _ := filepath.EvalSymlinks(wt.WorktreePath)
	cwdResolved, _ := filepath.EvalSymlinks(cwd)
	if cwdResolved != wtResolved {
		t.Errorf("CWD = %q, want %q", cwdResolved, wtResolved)
	}

	// Verify the worktree is a valid git work tree.
	if !gitutil.IsInsideWorkTree(wt.RepoRoot) {
		t.Error("worktree is not a valid git work tree")
	}

	// Verify the branch was created.
	branch, _ := gitutil.CurrentBranch(wt.RepoRoot)
	if branch == "main" || branch == "master" {
		t.Errorf("worktree branch should not be main/master, got %q", branch)
	}

	// Clean up the worktree so it doesn't persist after the test.
	_ = os.Chdir(origDir)
	_ = gitutil.RemoveWorktree(dir, wt.WorktreePath)
	_ = gitutil.DeleteBranch(dir, branch)
}
