package hook

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestMain clears the ambient XDG/plugin env so tests that fake $HOME resolve
// config and state under it, whatever the developer's own environment sets.
func TestMain(m *testing.M) {
	for _, v := range []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME", "CLAUDE_PLUGIN_DATA"} {
		_ = os.Unsetenv(v)
	}
	os.Exit(m.Run())
}

// Display output rides a different JSON field per event, and picking the wrong
// one silently shows the user nothing.
func TestStopEnvelopeIsSystemMessage(t *testing.T) {
	out, err := respondDisplay("Stop", &DisplayContent{Text: "art"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"systemMessage":"art"`) {
		t.Errorf("Stop must emit systemMessage, got %s", out)
	}
	if strings.Contains(out, `"systemMessage":"\n`) {
		t.Error("systemMessage must not begin with whitespace — it gets dropped")
	}
}

func TestMessageDisplayEnvelopeIsDisplayContent(t *testing.T) {
	out, err := respondDisplay("MessageDisplay", &DisplayContent{Text: "art"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"displayContent":"art"`) {
		t.Errorf("MessageDisplay must emit displayContent, got %s", out)
	}
}

func TestSystemMessageNeverStartsWithWhitespace(t *testing.T) {
	got := systemMessageText("\n\nbody")
	if strings.HasPrefix(got, "\n") || strings.HasPrefix(got, " ") {
		t.Errorf("Claude Code drops a systemMessage starting with whitespace: %q", got)
	}
}

func TestPostToolUseEnvelopeIsSystemMessage(t *testing.T) {
	out, err := respondDisplay("PostToolUse", &DisplayContent{Text: "body"})
	if err != nil {
		t.Fatal(err)
	}
	// A rule's own header already starts the text, so no label is prepended.
	if !strings.Contains(out, `"systemMessage":"body"`) {
		t.Errorf("want systemMessage carrying the text verbatim, got %s", out)
	}
}

// A findings block and an echo are independent: the model reads the reason, the
// user reads the systemMessage, and neither may suppress the other.
func TestEchoSurvivesAlongsideFindings(t *testing.T) {
	out, err := respond("PostToolUse", []Finding{{Rule: "lint", Message: "bad"}}, "echoed", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"decision":"block"`, `"systemMessage":`, "echoed"} {
		if !strings.Contains(out, want) {
			t.Errorf("envelope missing %q: %s", want, out)
		}
	}
}

// Advice is context, not a violation, so it must survive alongside a block:
// the model reads the reason AND the context.
func TestAdviceRidesAlongsideABlock(t *testing.T) {
	out, err := respond("PostToolUse", []Finding{{Rule: "r", Message: "bad"}}, "", "[lint] a note")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"decision":"block"`, `"additionalContext"`, "a note"} {
		if !strings.Contains(out, want) {
			t.Errorf("envelope missing %q: %s", want, out)
		}
	}
}

func TestAdviceAloneEmitsNoDecision(t *testing.T) {
	out, err := respondPassive("PostToolUse", "", "[lint] just context")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `"decision"`) {
		t.Errorf("advice alone must not block: %s", out)
	}
	if !strings.Contains(out, "just context") {
		t.Errorf("advice text missing: %s", out)
	}
}

// Every rule's advice is delivered, so joining must not drop any of them.
func TestJoinAdviceKeepsEveryRule(t *testing.T) {
	got := joinAdvice([]Advice{{Rule: "a", Text: "first"}, {Rule: "b", Text: "second"}})
	for _, want := range []string{"[a]", "first", "[b]", "second"} {
		if !strings.Contains(got, want) {
			t.Errorf("joined advice missing %q: %q", want, got)
		}
	}
	if joinAdvice(nil) != "" {
		t.Error("no advice must produce no text")
	}
}

// Stop used to short-circuit into the display path, so a Check rule registered
// on it ran never — silently, forever.
func TestStopChecksActuallyRun(t *testing.T) {
	withOnlyRule(t, Rule{
		ID: "stop-check", Events: []EventName{Stop}, EnabledByDefault: true,
		Check: func(context.Context, *Event) []Finding {
			return []Finding{{Message: "the tree is dirty"}}
		},
	})
	writeUserConfig(t, "")

	out, err := Decide(&Event{HookEventName: "Stop"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "the tree is dirty") {
		t.Errorf("a Check on Stop must reach the envelope, got %q", out)
	}
	if !strings.Contains(out, `"decision":"block"`) {
		t.Errorf("Stop blocks through the top-level decision, got %q", out)
	}
}

// Blocking a prompt must not reprint the prompt we refused to send.
func TestBlockedPromptIsSuppressed(t *testing.T) {
	out, err := respond("UserPromptSubmit", []Finding{{Rule: "r", Message: "nope"}}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"suppressOriginalPrompt":true`) {
		t.Errorf("want suppressOriginalPrompt on a blocked prompt, got %q", out)
	}
}

// A Gate on a non-PreToolUse event used to emit hookEventName "PreToolUse".
func TestGateEnvelopeNamesItsOwnEvent(t *testing.T) {
	out, err := respondDecision("PermissionRequest", &Decision{Permission: "deny", Reason: "no"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"hookEventName":"PermissionRequest"`) {
		t.Errorf("envelope must name the real event, got %q", out)
	}
}
