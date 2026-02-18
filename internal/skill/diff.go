package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pithecene-io/bonsai/internal/gitutil"
)

// BuildDiffPayload builds the diff payload for skill invocation,
// including synthetic diffs for untracked files. This matches the
// behavior in ai-skill.sh lines 258-286.
func BuildDiffPayload(repoRoot, baseRef string) (string, error) {
	if baseRef == "" || !gitutil.IsInsideWorkTree(repoRoot) {
		return "", nil
	}

	diffPayload, err := gitutil.Diff(repoRoot, baseRef)
	if err != nil {
		diffPayload = ""
	}

	// Append synthetic diffs for untracked files
	untracked, _ := gitutil.UntrackedFiles(repoRoot)
	if len(untracked) > 0 {
		diffPayload += BuildSyntheticUntrackedDiff(repoRoot, untracked)
	}

	return diffPayload, nil
}

// BuildSyntheticUntrackedDiff creates fake unified diff headers for
// untracked files. Matches ai-skill.sh lines 264-285.
func BuildSyntheticUntrackedDiff(repoRoot string, files []string) string {
	var buf strings.Builder
	for _, f := range files {
		fullPath := filepath.Join(repoRoot, f)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}

		// Detect file mode
		fmode := "100644"
		if info.Mode()&0o111 != 0 {
			fmode = "100755"
		}

		fmt.Fprintf(&buf, "\ndiff --git a/%s b/%s\n", f, f)
		fmt.Fprintf(&buf, "new file mode %s\n", fmode)
		buf.WriteString("--- /dev/null\n")
		fmt.Fprintf(&buf, "+++ b/%s\n", f)

		// Read file content for diff body
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		fmt.Fprintf(&buf, "@@ -0,0 +1,%d @@\n", len(lines))
		for _, line := range lines {
			buf.WriteString("+" + line + "\n")
		}
	}
	return buf.String()
}
