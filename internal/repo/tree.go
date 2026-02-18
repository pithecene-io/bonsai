package repo

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pithecene-io/bonsai/internal/gitutil"
)

// Tree returns the repository file listing: tracked + untracked files,
// sorted and deduplicated.
//
// In a git repo this matches:
//
//	{ git ls-files; git ls-files --others --exclude-standard; } | sort -u
//
// Outside a git repo it falls back to a filesystem walk, matching:
//
//	find . -type f | sed 's|^\./||' | sort
//
// Reference: ai-skill.sh:156-161
func Tree(dir string) ([]string, error) {
	if gitutil.IsInsideWorkTree(dir) {
		return gitTree(dir)
	}
	return findTree(dir)
}

// gitTree lists files using git ls-files (tracked + untracked).
func gitTree(dir string) ([]string, error) {
	tracked, err := gitutil.LsFiles(dir)
	if err != nil {
		return nil, err
	}

	untracked, err := gitutil.UntrackedFiles(dir)
	if err != nil {
		return nil, err
	}

	// Merge and deduplicate
	seen := make(map[string]struct{}, len(tracked)+len(untracked))
	var result []string
	for _, f := range tracked {
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			result = append(result, f)
		}
	}
	for _, f := range untracked {
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			result = append(result, f)
		}
	}

	sort.Strings(result)
	return result, nil
}

// findTree lists files using filepath.WalkDir (non-git fallback).
// Matches: find . -type f | sed 's|^\./||' | sort
func findTree(dir string) ([]string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	var result []string
	err = filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // WalkDir: skip unreadable entries
		}
		// Skip .git directories
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			rel, relErr := filepath.Rel(absDir, path)
			if relErr != nil {
				return nil //nolint:nilerr // skip entries with unresolvable paths
			}
			result = append(result, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(result)
	return result, nil
}

// TreeWithScope returns the repository tree filtered by scope prefixes.
// Each scope is a comma-separated path prefix. Files must start with
// at least one prefix to be included.
func TreeWithScope(dir, scope string) ([]string, error) {
	full, err := Tree(dir)
	if err != nil {
		return nil, err
	}

	if scope == "" {
		return full, nil
	}

	prefixes := strings.Split(scope, ",")
	for i := range prefixes {
		prefixes[i] = strings.TrimSpace(prefixes[i])
	}

	var filtered []string
	for _, f := range full {
		for _, p := range prefixes {
			if strings.HasPrefix(f, p) {
				filtered = append(filtered, f)
				break
			}
		}
	}

	return filtered, nil
}
