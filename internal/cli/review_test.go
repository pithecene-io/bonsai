package cli_test

import (
	"strings"
	"testing"

	"github.com/pithecene-io/bonsai/internal/cli"
)

func TestReview_CommandRegistered(t *testing.T) {
	app := cli.NewApp()
	var found bool
	for _, cmd := range app.Commands {
		if cmd.Name != "review" {
			continue
		}
		found = true

		// Usage must reflect autonomous semantics, not interactive session.
		if strings.Contains(strings.ToLower(cmd.Usage), "session") {
			t.Errorf("review Usage still says 'session': %q", cmd.Usage)
		}
		if strings.Contains(strings.ToLower(cmd.Usage), "codex") {
			t.Errorf("review Usage is provider-specific: %q", cmd.Usage)
		}
		break
	}
	if !found {
		t.Fatal("review command not registered in app")
	}
}
