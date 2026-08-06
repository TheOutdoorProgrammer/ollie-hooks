package mermaid

import (
	"context"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

// feed streams text through the rule in chunks and returns what the user ends
// up seeing. nil means "leave the screen alone", so the ORIGINAL delta shows —
// treating it as empty would make every test here claim the rule ate the text.
func feed(t *testing.T, session, message, text string, chunk int) string {
	t.Helper()
	if chunk <= 0 {
		chunk = len(text)
	}
	var shown strings.Builder
	for i := 0; i < len(text); i += chunk {
		end := min(i+chunk, len(text))
		delta := text[i:end]
		ev := hooktest.MessageDisplay(session, message, delta, end == len(text))
		if dc := displayMermaidStream(context.Background(), ev); dc != nil {
			shown.WriteString(dc.Text)
			continue
		}
		shown.WriteString(delta)
	}
	return shown.String()
}

const proseOnly = "Here is a sentence.\nAnd another one.\nNo diagrams at all.\n"

// Deltas split at arbitrary offsets, mid-line and even mid-fence-marker, so
// reassembly must produce identical output however the text is chopped up.
// This is the whole reason the rule keeps state.
func TestReassemblyIsIndependentOfChunkSize(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	text := "before\n```mermaid\nstateDiagram-v2\n  [*] --> Idle\n```\nafter\n"

	whole := feed(t, "s", "m0", text, 0)
	for _, size := range []int{1, 2, 3, 7, 13, 40} {
		got := feed(t, "s", "m"+strings.Repeat("x", size), text, size)
		if got != whole {
			t.Errorf("chunk size %d changed the output:\n got %q\nwant %q", size, got, whole)
		}
	}
}

// Text with no fence must survive byte-for-byte: this rule replaces the screen,
// so losing or gaining a character corrupts what the user reads.
func TestProseIsPassedThroughUnchanged(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	for _, size := range []int{0, 1, 5} {
		if got := feed(t, "s", "p", proseOnly, size); got != "" && got != proseOnly {
			t.Errorf("chunk %d mangled prose:\n got %q\nwant %q or empty", size, got, proseOnly)
		}
	}
}

// A fence that never closes would vanish with the state file, so the raw text
// is flushed on the final delta — half a diagram is worse than none, but
// silently eating the user's text is worse still.
func TestUnterminatedFenceIsFlushedRaw(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	text := "intro\n```mermaid\ngraph LR\n  A-->B\n"
	got := feed(t, "s", "unterminated", text, 4)
	if !strings.Contains(got, "graph LR") {
		t.Errorf("an unclosed fence must not swallow its content, got %q", got)
	}
}

// Two sessions stream concurrently through one global hook — same message id,
// different session — so their buffers must not mix.
func TestConcurrentMessagesDoNotShareState(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")

	// Leave session A mid-fence, then run a whole message through session B.
	openA := hooktest.MessageDisplay("sess-a", "msg-1", "```mermaid\ngraph LR\n", false)
	displayMermaidStream(context.Background(), openA)

	got := feed(t, "sess-b", "msg-1", "```mermaid\nstateDiagram-v2\n  [*] --> Idle\n```\n", 4)
	if strings.Contains(got, "graph LR") {
		t.Errorf("session A's open fence leaked into session B: %q", got)
	}
	if !strings.Contains(got, "stateDiagram-v2") {
		t.Errorf("session B lost its own content: %q", got)
	}
}

// Returning nil is how the rule says "show the original". Anything else on a
// plain delta would mean rewriting text it has no business touching.
func TestNilMeansUnchanged(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	ev := hooktest.MessageDisplay("s", "plain", "ordinary words", false)
	if dc := displayMermaidStream(context.Background(), ev); dc != nil {
		t.Errorf("a delta with nothing to render must return nil, got %q", dc.Text)
	}
}

// enabled is the registry's to enforce, so this asks the registry. Calling the
// rule directly would bypass the check and pass for the wrong reason.
func TestDisabledProducesNothing(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = false")
	ev := hooktest.MessageDisplay("s", "m", "```mermaid\ngraph LR\n A-->B\n```\n", true)
	if dc := hook.RunDisplays(ev); dc != nil {
		t.Errorf("disabled must produce nothing, got %q", dc.Text)
	}
}

// An event with no message id cannot be keyed to a buffer, so it must not try.
func TestMissingMessageIDIsIgnored(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	ev := hooktest.MessageDisplay("s", "", "```mermaid\n", false)
	if dc := displayMermaidStream(context.Background(), ev); dc != nil {
		t.Error("an event with no message id must be left alone")
	}
}

// The fast path: idle state plus a delta with no backtick cannot begin a fence.
func TestIdleDeltaWithoutBacktickIsSkipped(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	ev := hooktest.MessageDisplay("s", "quiet", "just words here", false)
	if dc := displayMermaidStream(context.Background(), ev); dc != nil {
		t.Errorf("a plain delta needs no rewrite, got %q", dc.Text)
	}
}

func TestRenderedFenceReplacesTheSource(t *testing.T) {
	requireMermaidASCII(t)
	hooktest.Config(t, RuleID, "enabled = true")
	got := feed(t, "s", "rendered", "```mermaid\ngraph LR\n  A[start] --> B[end]\n```\n", 6)
	if !strings.Contains(got, "start") || !strings.Contains(got, "┌") {
		t.Errorf("want box art carrying the node labels, got %q", got)
	}
	if strings.Contains(got, "graph LR") {
		t.Errorf("the mermaid source should be replaced by the render, got %q", got)
	}
}

// An unsupported diagram type cannot render, so the original must come back
// rather than the fence disappearing.
func TestUnrenderableFenceKeepsItsSource(t *testing.T) {
	requireMermaidASCII(t)
	hooktest.Config(t, RuleID, "enabled = true")
	got := feed(t, "s", "unsupported", "```mermaid\nstateDiagram-v2\n  [*] --> Idle\n```\n", 5)
	if !strings.Contains(got, "stateDiagram-v2") {
		t.Errorf("an unrenderable fence must keep its source, got %q", got)
	}
}
