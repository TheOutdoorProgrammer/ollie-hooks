package hook

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Clearing one rule's state must not delete another's. The sweep used to take
// the whole directory, so an unrelated rule's counters — and the binary cache —
// disappeared as a side effect.
func TestClearOnlySweepsTheCallingRule(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("CLAUDE_PLUGIN_DATA", "")

	stale := time.Now().Add(-48 * time.Hour)
	sd := stateDir()
	if err := os.MkdirAll(sd, 0o755); err != nil {
		t.Fatal(err)
	}
	others := []string{"other-rule.json", "other-rule-key.json", "binary-cache.json"}
	mine := []string{"mine.json", "mine-abc.json"}
	for _, name := range append(append([]string{}, others...), mine...) {
		p := filepath.Join(sd, name)
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, stale, stale); err != nil {
			t.Fatal(err)
		}
	}

	RuleState("mine").Clear()

	for _, name := range others {
		if _, err := os.Stat(filepath.Join(sd, name)); err != nil {
			t.Errorf("%s was swept by an unrelated rule's Clear", name)
		}
	}
	for _, name := range mine {
		if _, err := os.Stat(filepath.Join(sd, name)); err == nil {
			t.Errorf("%s is this rule's own stale state and should be gone", name)
		}
	}
}
