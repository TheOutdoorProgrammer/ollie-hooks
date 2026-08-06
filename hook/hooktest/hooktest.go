// Package hooktest provides fixtures for testing rules.
//
// Rules are pure functions over an Event, so testing one is mostly a matter of
// building a realistic event and pointing config lookups somewhere disposable.
// Both are fiddly enough to get subtly wrong, which is why they live here
// rather than in every rule's test file.
package hooktest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// Config redirects config lookups to a disposable home and writes body as the
// [rules.<id>] section. Clearing XDG_CONFIG_HOME matters: without it a
// developer who sets that variable gets different results from CI.
func Config(t *testing.T, ruleID, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("CLAUDE_PLUGIN_DATA", "")
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
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("CLAUDE_PLUGIN_DATA", "")
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
