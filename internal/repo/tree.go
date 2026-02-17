package repo

import (
	"sort"
	"strings"

	"github.com/justapithecus/bonsai/internal/gitutil"
)

// Tree returns the repository file listing: tracked + untracked files,
// sorted and deduplicated. This matches the shell behavior:
//
//	{ git ls-files; git ls-files --others --exclude-standard; } | sort -u
func Tree(dir string) ([]string, error) {
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
