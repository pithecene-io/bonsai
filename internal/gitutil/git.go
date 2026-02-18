// Package gitutil provides exec-based git command helpers.
// It has no internal dependencies and shells out to git directly.
package gitutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes a git command in the given directory and returns stdout.
// If dir is empty, the command runs in the current working directory.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// RunLines executes a git command and returns non-empty output lines.
func RunLines(dir string, args ...string) ([]string, error) {
	out, err := Run(dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// RevParse runs git rev-parse with the given arguments.
func RevParse(dir string, args ...string) (string, error) {
	full := append([]string{"rev-parse"}, args...)
	return Run(dir, full...)
}

// Diff returns the output of git diff with the given arguments.
func Diff(dir string, args ...string) (string, error) {
	full := append([]string{"diff"}, args...)
	return Run(dir, full...)
}

// DiffNameOnly returns file names from git diff --name-only.
func DiffNameOnly(dir, base string) ([]string, error) {
	return RunLines(dir, "diff", "--name-only", base)
}

// DiffNameStatus returns the output of git diff --name-status.
func DiffNameStatus(dir, base string) ([]string, error) {
	return RunLines(dir, "diff", "--name-status", base)
}

// DiffStat returns the output of git diff --stat.
func DiffStat(dir, base string) (string, error) {
	return Run(dir, "diff", "--stat", base)
}

// MergeBase returns the merge base between two refs.
func MergeBase(dir, ref1, ref2 string) (string, error) {
	return Run(dir, "merge-base", ref1, ref2)
}

// LsFiles returns tracked files matching the given arguments.
func LsFiles(dir string, args ...string) ([]string, error) {
	full := append([]string{"ls-files"}, args...)
	return RunLines(dir, full...)
}

// UntrackedFiles returns untracked, non-ignored files.
func UntrackedFiles(dir string) ([]string, error) {
	return LsFiles(dir, "--others", "--exclude-standard")
}

// IsInsideWorkTree returns true if the directory is inside a git work tree.
func IsInsideWorkTree(dir string) bool {
	out, err := RevParse(dir, "--is-inside-work-tree")
	return err == nil && out == "true"
}

// ShowToplevel returns the repository root directory.
func ShowToplevel(dir string) (string, error) {
	return RevParse(dir, "--show-toplevel")
}

// CurrentBranch returns the current branch name.
func CurrentBranch(dir string) (string, error) {
	return RevParse(dir, "--abbrev-ref", "HEAD")
}

// GitDir returns the .git directory path.
func GitDir(dir string) (string, error) {
	return RevParse(dir, "--git-dir")
}

// GitCommonDir returns the common .git directory (differs from GitDir in worktrees).
func GitCommonDir(dir string) (string, error) {
	return RevParse(dir, "--git-common-dir")
}

// RefExists returns true if the given ref exists.
func RefExists(dir, ref string) bool {
	_, err := Run(dir, "rev-parse", "--verify", ref)
	return err == nil
}
