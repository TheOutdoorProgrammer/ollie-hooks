package prosewrap

import (
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func TestAppliesHonoursExtensions(t *testing.T) {
	c := defaultConfig()
	if !c.applies("/x/notes.md") || !c.applies("/x/NOTES.MARKDOWN") {
		t.Error("markdown is the default, case-insensitively")
	}
	if c.applies("/x/main.go") {
		t.Error("source files are not prose")
	}

	// Someone writing .mdx or plain .txt gets the same renderer behaviour.
	c.Extensions = []string{".mdx", ".txt"}
	if !c.applies("/x/page.mdx") || c.applies("/x/page.md") {
		t.Error("a custom extension list must replace the default, not extend it")
	}
}

func TestAppliesRespectsIgnorePaths(t *testing.T) {
	c := defaultConfig()
	c.IgnorePaths = []string{"legacy/docs/"}
	if c.applies("/repo/legacy/docs/page.md") {
		t.Error("an ignored tree must be skipped even for a matching extension")
	}
	if !c.applies("/repo/current/page.md") {
		t.Error("other trees still apply")
	}
}

// An explicitly empty list in config would otherwise disable the rule silently
// by matching no extension at all.
func TestLoadConfigRepairsEmptyValues(t *testing.T) {
	hooktest.Config(t, RuleID, "extensions = []\nmax_reported = 0\n")
	got := loadConfig()
	if len(got.Extensions) == 0 {
		t.Error("an empty extension list must fall back to the default")
	}
	if got.MaxReported <= 0 {
		t.Errorf("a non-positive cap must fall back, got %d", got.MaxReported)
	}
}
