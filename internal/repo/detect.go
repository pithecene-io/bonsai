// Package repo provides repository detection, context, and tree listing.
package repo

import (
	"fmt"

	"github.com/justapithecus/bonsai/internal/gitutil"
)

// Info holds detected repository metadata.
type Info struct {
	Root      string // Absolute path to repo root
	Branch    string // Current branch name
	MergeBase string // Merge base commit (may be empty)
	IsWorktree bool  // True if running in a git worktree (not main checkout)
}

// MergeBaseCandidates are tried in order to find a merge base.
var MergeBaseCandidates = []string{"main", "master", "origin/main", "origin/master"}

// Detect detects repository metadata from the given directory.
// If dir is empty, uses the current working directory.
func Detect(dir string) (*Info, error) {
	if !gitutil.IsInsideWorkTree(dir) {
		return nil, fmt.Errorf("not inside a git work tree: %s", dir)
	}

	root, err := gitutil.ShowToplevel(dir)
	if err != nil {
		return nil, fmt.Errorf("show toplevel: %w", err)
	}

	branch, err := gitutil.CurrentBranch(dir)
	if err != nil {
		return nil, fmt.Errorf("current branch: %w", err)
	}

	info := &Info{
		Root:       root,
		Branch:     branch,
		IsWorktree: isWorktree(dir),
	}

	return info, nil
}

// DetectMergeBase finds the merge base between HEAD and the first
// available candidate ref. Uses the provided candidates, or defaults
// to MergeBaseCandidates if nil.
func DetectMergeBase(dir string, candidates []string) string {
	if candidates == nil {
		candidates = MergeBaseCandidates
	}
	for _, candidate := range candidates {
		if !gitutil.RefExists(dir, candidate) {
			continue
		}
		mb, err := gitutil.MergeBase(dir, candidate, "HEAD")
		if err == nil && mb != "" {
			return mb
		}
	}
	return ""
}

// isWorktree returns true if the directory is a git worktree
// (not the main checkout). In a worktree, git-common-dir differs
// from git-dir.
func isWorktree(dir string) bool {
	gitDir, err := gitutil.GitDir(dir)
	if err != nil {
		return false
	}
	commonDir, err := gitutil.GitCommonDir(dir)
	if err != nil {
		return false
	}
	return gitDir != commonDir
}
