package hook

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeUserConfig points config lookups at a disposable home. hooktest cannot
// be used here: it imports this package.
func writeUserConfig(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// os.UserHomeDir reads USERPROFILE on Windows, so HOME alone would leave
	// these tests reading the real user's config.
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("CLAUDE_PLUGIN_DATA", "")
	cfgDir := filepath.Join(dir, ".config", "ollie-hooks")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if body == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// withOnlyRule swaps the registry for the duration of a test, so registering a
// fixture cannot leak into whatever runs next.
func withOnlyRule(t *testing.T, r Rule) {
	t.Helper()
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil
	Register(r)
}

func TestSeverityRoutesFindings(t *testing.T) {
	withOnlyRule(t, Rule{
		ID: "sev-demo", Events: []EventName{PostToolUse}, EnabledByDefault: true,
		Check: func(context.Context, *Event) []Finding {
			return []Finding{{Message: "something to say"}}
		},
	})
	ev := &Event{HookEventName: "PostToolUse"}

	t.Run("block is the default", func(t *testing.T) {
		writeUserConfig(t, "")
		res := RunChecks(ev)
		if len(res.Findings) != 1 || len(res.Advisories) != 0 {
			t.Errorf("want one blocking finding, got %+v", res)
		}
	})

	t.Run("advisory downgrades to context", func(t *testing.T) {
		writeUserConfig(t, "[rules.sev-demo]\nseverity = \"advisory\"\n")
		res := RunChecks(ev)
		if len(res.Findings) != 0 {
			t.Errorf("advisory must not block, got findings %+v", res.Findings)
		}
		if len(res.Advisories) != 1 || !strings.Contains(res.Advisories[0].Text, "something to say") {
			t.Errorf("advisory should carry the same text, got %+v", res.Advisories)
		}
	})

	t.Run("off skips entirely", func(t *testing.T) {
		writeUserConfig(t, "[rules.sev-demo]\nseverity = \"off\"\n")
		if res := RunChecks(ev); len(res.Findings)+len(res.Advisories) != 0 {
			t.Errorf("off must produce nothing, got %+v", res)
		}
	})

	// A typo must not silently disable a check the user believes is running.
	t.Run("an unrecognised value falls back", func(t *testing.T) {
		writeUserConfig(t, "[rules.sev-demo]\nseverity = \"blahck\"\n")
		if res := RunChecks(ev); len(res.Findings) != 1 {
			t.Errorf("unknown severity should fall back to the default, got %+v", res)
		}
	})
}

// A rule may ship as advisory, and the user can still promote it to blocking.
func TestDefaultSeverityIsOverridable(t *testing.T) {
	withOnlyRule(t, Rule{
		ID: "advisory-demo", Events: []EventName{PostToolUse}, EnabledByDefault: true,
		DefaultSeverity: SeverityAdvisory,
		Check: func(context.Context, *Event) []Finding {
			return []Finding{{Message: "fyi"}}
		},
	})
	ev := &Event{HookEventName: "PostToolUse"}

	writeUserConfig(t, "")
	if res := RunChecks(ev); len(res.Advisories) != 1 || len(res.Findings) != 0 {
		t.Errorf("an advisory-by-default rule should advise, got %+v", res)
	}
	writeUserConfig(t, "[rules.advisory-demo]\nseverity = \"block\"\n")
	if res := RunChecks(ev); len(res.Findings) != 1 {
		t.Errorf("the user must be able to promote it to blocking, got %+v", res)
	}
}
