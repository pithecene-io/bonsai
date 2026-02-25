package agent

import "testing"

func TestResolveModel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"haiku", "claude-haiku-4-5-20251001"},
		{"sonnet", "claude-sonnet-4-6"},
		{"opus", "claude-opus-4-6"},
		{"claude-haiku-4-5-20251001", "claude-haiku-4-5-20251001"},
		{"claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"unknown-model", "unknown-model"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := resolveModel(tt.input)
			if got != tt.want {
				t.Errorf("resolveModel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProfileFor(t *testing.T) {
	tests := []struct {
		tier    string
		wantMax int64
	}{
		{"haiku", 4096},
		{"sonnet", 8192},
		{"opus", 8192},
		{"unknown", 8192}, // falls back to sonnet
	}
	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			p := profileFor(tt.tier)
			if p.maxTokens != tt.wantMax {
				t.Errorf("profileFor(%q).maxTokens = %d, want %d", tt.tier, p.maxTokens, tt.wantMax)
			}
		})
	}
}
