package cli

import "testing"

func TestLooksLikePlan(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "real plan with Go file paths",
			text: `## Files to modify

1. **internal/agent/claude.go** — remove --tools flag
2. **internal/cli/patch.go** — add plan detection

### Assertions
- Tests pass after modification`,
			want: true,
		},
		{
			name: "real plan with relative paths",
			text: "Modify ./src/index.ts to export the new handler",
			want: true,
		},
		{
			name: "real plan with backtick path",
			text: "Edit `config/settings.yaml` to add the new key",
			want: true,
		},
		{
			name: "clarification request",
			text: `I don't have access to the GitHub URL directly. To plan this patch accurately, I need the issue content.

Could you paste the text of issue #163 here? Specifically:

- The **title** of the issue
- The **description** (what behavior is wrong or missing)
- Any **reproduction steps** or **expected vs actual** behavior noted in the issue`,
			want: false,
		},
		{
			name: "cannot access URL",
			text: `I can't reach the GitHub URL. Could you provide the issue details?`,
			want: false,
		},
		{
			name: "empty response",
			text: "",
			want: false,
		},
		{
			name: "generic refusal",
			text: "I need more information about what you'd like me to change.",
			want: false,
		},
		{
			name: "plan with Python paths",
			text: "Modify src/handlers/auth.py to fix the token refresh logic",
			want: true,
		},
		{
			name: "plan with nested paths",
			text: "Update internal/config/load.go and internal/config/config.go",
			want: true,
		},
		{
			name: "clarification with GitHub URL",
			text: `I can't access the URL directly. Could you paste the contents of https://github.com/pithecene-io/lode/issues/163 so I can plan the patch?`,
			want: false,
		},
		{
			name: "clarification with multiple URLs",
			text: `I need more context. Please share the details from:
- https://github.com/pithecene-io/bonsai/issues/42
- https://linear.app/pithecene/issue/PIT-123

Without the issue content I cannot produce a plan.`,
			want: false,
		},
		{
			name: "clarification mentioning github.com domain",
			text: `I don't have access to github.com URLs. Could you copy the issue description here?`,
			want: false,
		},
		{
			name: "real plan that also mentions a URL",
			text: `Based on https://github.com/pithecene-io/lode/issues/163:

## Files to modify
1. **internal/agent/claude.go** — add read-only tool policy
2. **internal/cli/patch.go** — pass ToolsReadOnly in architect phase`,
			want: true,
		},
		{
			name: "plan targeting CHANGELOG.md",
			text: "Update CHANGELOG.md to add the v0.1.1 release entry",
			want: true,
		},
		{
			name: "plan targeting dotfile",
			text: "Modify `.goreleaser.yaml` to disable the changelog generation",
			want: true,
		},
		{
			name: "plan targeting multiple root files",
			text: `## Files to modify
1. **CHANGELOG.md** — add v0.2.0 entry
2. **README.md** — update installation instructions
3. **CLAUDE.md** — add new dependency rule`,
			want: true,
		},
		{
			name: "plan targeting AGENTS.md",
			text: "Edit AGENTS.md to add the new structural rule about worktrees",
			want: true,
		},
		{
			name: "plan targeting .bonsai.yaml",
			text: "Add a `routing` section to .bonsai.yaml with merge-base candidates",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikePlan(tt.text); got != tt.want {
				t.Errorf("looksLikePlan() = %v, want %v", got, tt.want)
			}
		})
	}
}
