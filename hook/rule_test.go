package hook

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExpandFindings(t *testing.T) {
	in := []Finding{
		{Rule: "single", Message: "one line"},
		{Rule: "multi", Message: "first\nsecond\nthird"},
	}
	got := expandFindings(in, 100)
	want := []Finding{
		{Rule: "single", Message: "one line"},
		{Rule: "multi", Message: "first"},
		{Rule: continuationGlyph, Message: "second"},
		{Rule: continuationGlyph, Message: "third"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Rules write messages as indented raw strings; source indentation must never
// reach the screen, and no row may exceed the configured budget.
func TestExpandFindingsNormalisesAndWraps(t *testing.T) {
	in := []Finding{{Rule: "r", Message: `
		one two three four five
		six seven eight nine ten`}}
	got := expandFindings(in, 20)
	if len(got) < 2 {
		t.Fatalf("a message past the width must wrap, got %+v", got)
	}
	for _, f := range got {
		if len([]rune(f.Message)) > 20 {
			t.Errorf("row exceeds width: %q", f.Message)
		}
		if strings.Contains(f.Message, "\t") || strings.HasPrefix(f.Message, " ") {
			t.Errorf("source indentation leaked: %q", f.Message)
		}
	}
	if got[0].Rule != "r" {
		t.Errorf("first row keeps the rule name, got %q", got[0].Rule)
	}
	if got[1].Rule != continuationGlyph {
		t.Errorf("later rows take the continuation marker, got %q", got[1].Rule)
	}
}

// A word longer than the budget stays intact: half a URL beats a long row.
func TestExpandFindingsKeepsLongWordsWhole(t *testing.T) {
	url := "https://example.com/a/very/long/path/that/exceeds/the/width"
	got := expandFindings([]Finding{{Rule: "r", Message: url}}, 20)
	if len(got) != 1 || got[0].Message != url {
		t.Errorf("long word must not be split, got %+v", got)
	}
}

func TestRunWithTimeout(t *testing.T) {
	if done := runWithTimeout(1, func(context.Context) {}); !done {
		t.Error("fast fn should finish within its window")
	}

	// A fn that outlives its window reports not-done (fail-open). It sleeps past
	// the deadline unconditionally so the result is deterministic; ctx
	// cancellation itself is guaranteed by context.WithTimeout.
	done := runWithTimeout(1, func(context.Context) { time.Sleep(2 * time.Second) })
	if done {
		t.Error("overrunning fn should report not-done (fail-open)")
	}
}
