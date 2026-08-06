package hook

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// Hand-building an updatedInput is how a rewrite silently destroys a Write:
// updatedInput replaces the whole object, so an omitted `content` is a lost file.
func TestMergeInputKeepsUntouchedFields(t *testing.T) {
	ev := &Event{ToolInput: json.RawMessage(`{"file_path":"/a.go","content":"package a","dry_run":true}`)}
	got := ev.MergeInput(map[string]any{"file_path": "/b.go"})

	if got["file_path"] != "/b.go" {
		t.Errorf("the change should apply, got %v", got["file_path"])
	}
	for _, k := range []string{"content", "dry_run"} {
		if _, ok := got[k]; !ok {
			t.Errorf("field %q was dropped from the rewritten input", k)
		}
	}
}

func TestMergeInputHandlesAbsentInput(t *testing.T) {
	ev := &Event{}
	if got := ev.MergeInput(map[string]any{"x": 1}); got["x"] != 1 {
		t.Errorf("a missing tool_input must still accept changes, got %+v", got)
	}
}

// Over the cap Claude Code swaps the content for a FILE PATH, so the model
// would receive a filename where it expected findings.
func TestFitFindingsStaysUnderTheOutputCap(t *testing.T) {
	encode := func(fs []Finding) (string, error) {
		var b strings.Builder
		for _, f := range fs {
			b.WriteString(f.Rule + "," + f.Message + "\n")
		}
		return b.String(), nil
	}
	big := make([]Finding, 400)
	for i := range big {
		big[i] = Finding{Rule: "lint", Message: strings.Repeat("x", 60) + strconv.Itoa(i)}
	}
	kept, out, err := fitFindings(big, encode)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > maxOutputChars {
		t.Errorf("encoded reason is %d chars, over the %d cap", len(out), maxOutputChars)
	}
	if len(kept) >= len(big) {
		t.Error("expected findings to be trimmed")
	}
	if !strings.Contains(out, "omitted") {
		t.Errorf("the model must be told findings were dropped:\n%s", out[max(0, len(out)-200):])
	}
}

func TestFitFindingsLeavesSmallSetsAlone(t *testing.T) {
	encode := func(fs []Finding) (string, error) { return strings.Repeat("a", len(fs)), nil }
	in := []Finding{{Rule: "r", Message: "one"}}
	kept, _, err := fitFindings(in, encode)
	if err != nil || len(kept) != 1 {
		t.Errorf("a small set must pass through untouched, got %d (%v)", len(kept), err)
	}
}
