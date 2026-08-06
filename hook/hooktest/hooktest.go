// Package hooktest provides fixtures for testing rules.
//
// Rules are pure functions over an Event, so testing one is mostly a matter of
// building a realistic event and pointing config lookups somewhere disposable.
// Both are fiddly enough to get subtly wrong, which is why they live here
// rather than in every rule's test file.
package hooktest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// Home points every config and state lookup at dir. USERPROFILE goes with HOME
// because os.UserHomeDir reads that one on Windows, where setting HOME alone
// leaves the test quietly reading the real user's config.
func Home(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("CLAUDE_PLUGIN_DATA", "")
}

// Config redirects config lookups to a disposable home and writes body as the
// [rules.<id>] section.
func Config(t *testing.T, ruleID, body string) {
	t.Helper()
	dir := t.TempDir()
	Home(t, dir)
	cfgDir := filepath.Join(dir, ".config", "ollie-hooks")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "[rules." + ruleID + "]\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// NoConfig points lookups at an empty home, so a rule uses its defaults.
func NoConfig(t *testing.T) {
	t.Helper()
	Home(t, t.TempDir())
}

func mustJSON(t *testing.T, v map[string]any) []byte {
	t.Helper()
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// PreToolUse builds an event for a tool call about to run.
func PreToolUse(t *testing.T, tool string, input map[string]any) *hook.Event {
	t.Helper()
	return &hook.Event{
		HookEventName: "PreToolUse",
		ToolName:      tool,
		ToolInput:     mustJSON(t, input),
	}
}

// PostToolUse builds an event for a tool call that succeeded.
func PostToolUse(t *testing.T, tool string, input, response map[string]any) *hook.Event {
	t.Helper()
	return &hook.Event{
		HookEventName: "PostToolUse",
		ToolName:      tool,
		ToolInput:     mustJSON(t, input),
		ToolResponse:  mustJSON(t, response),
	}
}

// PostToolUseFailure builds an event for a failed tool call. Output arrives in
// Error, not ToolResponse — a failure carries no tool_response at all.
func PostToolUseFailure(t *testing.T, tool string, input map[string]any, errText string) *hook.Event {
	t.Helper()
	return &hook.Event{
		HookEventName: "PostToolUseFailure",
		ToolName:      tool,
		ToolInput:     mustJSON(t, input),
		Error:         errText,
	}
}

// MessageDisplay builds one streamed delta of an assistant message.
func MessageDisplay(sessionID, messageID, delta string, final bool) *hook.Event {
	return &hook.Event{
		HookEventName: "MessageDisplay",
		SessionID:     sessionID,
		MessageID:     messageID,
		Delta:         delta,
		Final:         final,
	}
}

// AssertClean fails when any finding fired.
func AssertClean(t *testing.T, findings []hook.Finding) {
	t.Helper()
	if len(findings) > 0 {
		t.Errorf("expected no findings, got %d: %s", len(findings), Messages(findings))
	}
}

// AssertFinding fails unless some finding's message contains want.
func AssertFinding(t *testing.T, findings []hook.Finding, want string) {
	t.Helper()
	if len(findings) == 0 {
		t.Fatalf("expected a finding containing %q, got none", want)
	}
	for _, f := range findings {
		if strings.Contains(f.Message, want) {
			return
		}
	}
	t.Errorf("no finding contained %q, got: %s", want, Messages(findings))
}

// Messages joins finding messages for failure output.
func Messages(findings []hook.Finding) string {
	msgs := make([]string, 0, len(findings))
	for _, f := range findings {
		msgs = append(msgs, f.Rule+": "+f.Message)
	}
	return strings.Join(msgs, " | ")
}

// AssertConfigDocumented checks the generated example against every registered
// rule, in both directions: each documented key must decode into that rule's
// config, and each rule must have no decodable key the docs never mention.
//
// This is the check prose review cannot do. A key renamed in a struct tag, or
// added without a doc tag, produces docs that read perfectly and describe a
// config nobody can use.
func AssertConfigDocumented(t *testing.T) {
	t.Helper()

	var buf bytes.Buffer
	hook.WriteConfigExample(&buf)
	live := hook.UncommentExample(buf.String())

	var root struct {
		Rules       map[string]toml.Primitive  `toml:"rules"`
		Output      toml.Primitive             `toml:"output"`
		CustomRules map[string]hook.CustomRule `toml:"custom_rules"`
	}
	md, err := toml.Decode(live, &root)
	if err != nil {
		t.Fatalf("generated example is not valid TOML: %v\n%s", err, live)
	}

	// Framework-owned sections, decoded so they do not read as undocumented.
	var output struct {
		WrapWidth int `toml:"wrap_width"`
	}
	if err := md.PrimitiveDecode(root.Output, &output); err != nil {
		t.Errorf("[output] does not decode: %v", err)
	}

	for _, r := range hook.Registered() {
		section, ok := root.Rules[r.ID]
		if !ok {
			t.Errorf("%s: no [rules.%s] section in the generated example", r.ID, r.ID)
			continue
		}

		// enabled/severity/timeout belong to the framework, not to the rule's
		// own struct, so they need decoding separately or they look stray.
		var universal struct {
			Enabled  *bool   `toml:"enabled"`
			Severity *string `toml:"severity"`
			Timeout  *int    `toml:"timeout"`
		}
		if err := md.PrimitiveDecode(section, &universal); err != nil {
			t.Errorf("%s: universal keys do not decode: %v", r.ID, err)
		}

		if r.Config == nil {
			continue
		}
		// A fresh zero value, not r.Config: decoding into the defaults would
		// hide a key that silently fails to apply.
		fresh := reflect.New(reflect.TypeOf(r.Config))
		if err := md.PrimitiveDecode(section, fresh.Interface()); err != nil {
			t.Errorf("%s: documented config does not decode: %v", r.ID, err)
		}
	}

	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		var stray []string
		for _, k := range undecoded {
			stray = append(stray, k.String())
		}
		t.Errorf("the example documents keys nothing reads: %s", strings.Join(stray, ", "))
	}
}
