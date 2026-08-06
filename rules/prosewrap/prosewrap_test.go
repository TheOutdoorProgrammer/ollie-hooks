package prosewrap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func pwWrite(path, content string) *hook.Event {
	in, _ := json.Marshal(map[string]any{"file_path": path, "content": content})
	return &hook.Event{HookEventName: "PostToolUse", ToolName: "Write", ToolInput: in}
}

func pwEdit(path, oldS, newS string) *hook.Event {
	in, _ := json.Marshal(map[string]any{"file_path": path, "old_string": oldS, "new_string": newS})
	return &hook.Event{HookEventName: "PostToolUse", ToolName: "Edit", ToolInput: in}
}

const wrapped = "This is a sentence that someone\nwrapped by hand.\n"

func TestCheckProseWrapWrite(t *testing.T) {
	hooktest.NoConfig(t) // no config file → built-in defaults

	if got := checkProseWrap(context.Background(), pwWrite("notes.md", wrapped)); len(got) == 0 {
		t.Fatal("want findings for hand-wrapped markdown, got none")
	}
	if got := checkProseWrap(context.Background(), pwWrite("notes.md", "One sentence.\nAnother one.\n")); got != nil {
		t.Fatalf("want no findings for sentence-per-line, got %v", hooktest.Messages(got))
	}
}

func TestProseWrapEmDashFlag(t *testing.T) {
	emDash := "This clause is set off with an em-dash — like this.\n"

	// Off by default: an em-dash alone is not flagged.
	hooktest.NoConfig(t)
	if got := checkProseWrap(context.Background(), pwWrite("notes.md", emDash)); got != nil {
		t.Fatalf("the em-dash flag is opt-in; want no findings by default, got %v", hooktest.Messages(got))
	}

	// Opt in, and the em-dash line is flagged.
	hooktest.Config(t, RuleID, "enabled = true\nflag_em_dashes = true")
	hooktest.AssertFinding(t, checkProseWrap(context.Background(), pwWrite("notes.md", emDash)), "em-dash")
}

// The rule is markdown-only: source files have their own comment budget, and a
// long line there is the comments rule's business, not this one's.
func TestCheckProseWrapOnlyMarkdown(t *testing.T) {
	hooktest.NoConfig(t)

	for _, path := range []string{"main.go", "script.sh", "config.yaml", "README.txt"} {
		if got := checkProseWrap(context.Background(), pwWrite(path, wrapped)); got != nil {
			t.Errorf("%s: want no findings, got %v", path, hooktest.Messages(got))
		}
	}
	for _, path := range []string{"notes.md", "NOTES.MD", "doc.markdown"} {
		if got := checkProseWrap(context.Background(), pwWrite(path, wrapped)); len(got) == 0 {
			t.Errorf("%s: want findings, got none", path)
		}
	}
}

// Only the text an Edit introduced is judged, so editing one paragraph never
// reports the rest of an already-wrapped file.
func TestCheckProseWrapScopedToTheEdit(t *testing.T) {
	hooktest.NoConfig(t)

	if got := checkProseWrap(context.Background(), pwEdit("notes.md", wrapped, "One clean sentence.\n")); got != nil {
		t.Fatalf("want no findings when the new text is clean, got %v", hooktest.Messages(got))
	}
	if got := checkProseWrap(context.Background(), pwEdit("notes.md", "One clean sentence.\n", wrapped)); len(got) == 0 {
		t.Fatal("want findings when the new text is wrapped, got none")
	}
}

func TestCheckProseWrapMultiEdit(t *testing.T) {
	hooktest.NoConfig(t)

	in, _ := json.Marshal(map[string]any{
		"file_path": "notes.md",
		"edits": []map[string]string{
			{"old_string": "a", "new_string": "Clean sentence.\n"},
			{"old_string": "b", "new_string": wrapped},
		},
	})
	ev := &hook.Event{HookEventName: "PostToolUse", ToolName: "MultiEdit", ToolInput: in}
	if got := checkProseWrap(context.Background(), ev); len(got) == 0 {
		t.Fatal("want findings from the wrapped edit, got none")
	}
}

func TestCheckProseWrapConfig(t *testing.T) {
	write := func(t *testing.T, body string) {
		t.Helper()
		hooktest.Config(t, RuleID, body)
	}

	// enabled is the framework's concern, so this has to go through the
	// registry — calling the check directly would bypass the very thing under
	// test and pass for the wrong reason.
	t.Run("disabled", func(t *testing.T) {
		write(t, "enabled = false\n")
		if got := hook.RunChecks(pwWrite("notes.md", wrapped)).Findings; got != nil {
			t.Fatalf("want no findings when disabled, got %v", hooktest.Messages(got))
		}
	})

	// The rule ships advisory, so its output arrives as context rather than a
	// refusal — hook.Run returns only what blocks.
	t.Run("enabled arrives as advice", func(t *testing.T) {
		write(t, "enabled = true\n")
		res := hook.RunChecks(pwWrite("notes.md", wrapped))
		if len(res.Findings) != 0 {
			t.Errorf("a style opinion should not block, got %+v", res.Findings)
		}
		if len(res.Advisories) == 0 {
			t.Fatal("want advice once enabled")
		}
	})

	t.Run("promoted to blocking on request", func(t *testing.T) {
		write(t, "enabled = true\nseverity = \"block\"\n")
		if got := hook.RunChecks(pwWrite("notes.md", wrapped)).Findings; len(got) == 0 {
			t.Fatal("the user must be able to make it block")
		}
	})

	// For trees that are still hard-wrapped and would otherwise flag constantly.
	t.Run("ignored path", func(t *testing.T) {
		write(t, "ignore_paths = [\"legacy/docs/\"]\n")
		if got := checkProseWrap(context.Background(), pwWrite("/repo/legacy/docs/page.md", wrapped)); got != nil {
			t.Fatalf("want no findings for an ignored path, got %v", hooktest.Messages(got))
		}
		if got := checkProseWrap(context.Background(), pwWrite("/repo/current/page.md", wrapped)); len(got) == 0 {
			t.Fatal("want findings for a path outside the ignore list, got none")
		}
	})
}

// Guidance is written for readability in source; the framework normalises and
// wraps it, so the only requirement here is that it says something.
func TestProseGuidanceIsNotEmpty(t *testing.T) {
	if g := strings.TrimSpace(proseGuidance()); g == "" {
		t.Error("guidance must carry text for the model to act on")
	}
}
