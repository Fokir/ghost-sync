package hooks

import (
	"strings"
	"testing"
)

func TestPostCommitScript(t *testing.T) {
	script := PostCommitScript()

	checks := []string{
		"ghost-sync",
		"command -v ghost-sync",
		"exit 0",
	}
	for _, want := range checks {
		if !strings.Contains(script, want) {
			t.Errorf("PostCommitScript: expected to contain %q", want)
		}
	}
}

func TestPostMergeScript(t *testing.T) {
	script := PostMergeScript()

	checks := []string{
		"ghost-sync pull",
		"command -v ghost-sync",
	}
	for _, want := range checks {
		if !strings.Contains(script, want) {
			t.Errorf("PostMergeScript: expected to contain %q", want)
		}
	}
}
