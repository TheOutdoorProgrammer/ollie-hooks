package hook

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// withRules swaps the registry for a test, restoring it after.
func withRules(t *testing.T, rules ...Rule) {
	t.Helper()
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil
	for _, r := range rules {
		Register(r)
	}
}

// writeSettings points claudeSettingsPath at a disposable dir and writes body
// as settings.json. An empty body writes no file, so the lookup misses.
func writeSettings(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	if body == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func checkRule(id string, ev EventName) Rule {
	return Rule{
		ID: id, Events: []EventName{ev},
		Check: func(context.Context, *Event) []Finding { return nil },
	}
}

func TestWiringAllEventsWired(t *testing.T) {
	withRules(t, checkRule("a", PostToolUse), checkRule("b", PreToolUse))
	writeSettings(t, `{"hooks":{
		"PostToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"/x/ollie-hooks"}]}],
		"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"/x/ollie-hooks"}]}]
	}}`)

	d := CheckWiring()
	if !d.SettingsFound {
		t.Fatal("settings.json should be found")
	}
	if !d.Clean() {
		t.Errorf("expected no drift, got missing %v", d.Missing)
	}
}

func TestWiringReportsUnwiredEvent(t *testing.T) {
	withRules(t, checkRule("a", PostToolUse), checkRule("b", PostToolUseFailure))
	writeSettings(t, `{"hooks":{
		"PostToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"/x/ollie-hooks"}]}]
	}}`)

	d := CheckWiring()
	if d.Clean() {
		t.Fatal("PostToolUseFailure is unwired; expected drift")
	}
	if !slices.Contains(d.Missing, PostToolUseFailure) {
		t.Errorf("PostToolUseFailure should be missing, got %v", d.Missing)
	}
	if slices.Contains(d.Missing, PostToolUse) {
		t.Errorf("PostToolUse is wired; it must not be reported, got %v", d.Missing)
	}
}

// A hook that belongs to another tool does not count as wiring ours: the whole
// point is to catch an event where ollie-hooks specifically never runs.
func TestWiringForeignHookDoesNotCount(t *testing.T) {
	withRules(t, checkRule("a", UserPromptSubmit))
	writeSettings(t, `{"hooks":{
		"UserPromptSubmit":[{"matcher":"","hooks":[{"type":"command","command":"/opt/other-tool run"}]}]
	}}`)

	if d := CheckWiring(); !slices.Contains(d.Missing, UserPromptSubmit) {
		t.Errorf("a foreign hook must not wire our event, got %v", d.Missing)
	}
}

func TestWiringMissingSettingsReportsEverything(t *testing.T) {
	withRules(t, checkRule("a", PostToolUse))
	writeSettings(t, "")

	d := CheckWiring()
	if d.SettingsFound {
		t.Error("no settings file should mean SettingsFound is false")
	}
	if d.Clean() {
		t.Error("nothing is wired; expected drift")
	}
}

// The command may carry arguments and a full path, so matching is by basename.
func TestInvokesOllieHooks(t *testing.T) {
	cases := map[string]bool{
		"/Users/j/bin/ollie-hooks":       true,
		"ollie-hooks":                    true,
		"/x/ollie-hooks hook claude":     true,
		"/x/ollie-hooks-helper":          false,
		"'/Library/Muxy/muxy-hook.sh' x": false,
	}
	for cmd, want := range cases {
		if got := invokesOllieHooks(cmd); got != want {
			t.Errorf("invokesOllieHooks(%q) = %v, want %v", cmd, got, want)
		}
	}
}
