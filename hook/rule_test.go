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

// A SingleFailure rule that fires returns its findings alone, so the model gets
// one precise diagnosis instead of every matching rule's output piled together.
func TestSingleFailureShortCircuits(t *testing.T) {
	writeUserConfig(t, "")
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = []Rule{
		{
			ID: "precise", Events: []EventName{PostToolUse}, EnabledByDefault: true,
			SingleFailure: true,
			Check:         func(context.Context, *Event) []Finding { return []Finding{{Message: "the one thing"}} },
		},
		{
			ID: "noisy", Events: []EventName{PostToolUse}, EnabledByDefault: true,
			Check: func(context.Context, *Event) []Finding { return []Finding{{Message: "also this"}} },
		},
	}
	res := RunChecks(&Event{HookEventName: "PostToolUse"})
	if len(res.Findings) != 1 || res.Findings[0].Message != "the one thing" {
		t.Errorf("SingleFailure must return only its own findings, got %+v", res.Findings)
	}
}

// A gate that overruns denies when it is fail-closed: the review-gate case,
// where letting the call through unreviewed is itself the failure.
func TestGateFailsClosedOnTimeout(t *testing.T) {
	writeUserConfig(t, "")
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = []Rule{{
		ID: "slow-gate", Events: []EventName{PreToolUse}, EnabledByDefault: true,
		FailClosedOnTimeout: true, Timeout: 1,
		Gate: func(context.Context, *Event) *Decision {
			time.Sleep(2 * time.Second)
			return &Decision{Permission: PermissionAllow}
		},
	}}
	dec := RunGates(&Event{HookEventName: "PreToolUse"})
	if dec == nil || dec.Permission != PermissionDeny {
		t.Errorf("a timed-out fail-closed gate must deny, got %+v", dec)
	}
}

// FailClosedOnTimeout only makes sense where a rule can block, so setting it on
// a Display rule must panic at registration rather than silently do nothing.
func TestFailClosedOnTimeoutRejectedOnNonBlockingVerb(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil
	defer func() {
		if recover() == nil {
			t.Error("FailClosedOnTimeout on a Display rule must panic")
		}
	}()
	Register(Rule{
		ID: "bad-failclosed", Events: []EventName{MessageDisplay},
		FailClosedOnTimeout: true,
		Display:             func(context.Context, *Event) *DisplayContent { return nil },
	})
}

// sandbox swaps the registry for the duration of a test and points config at an
// empty home, so runnable() sees the fixture enabled and nothing else.
func sandbox(t *testing.T, rules []Rule) {
	t.Helper()
	writeUserConfig(t, "")
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = rules
}

func preEvent() *Event {
	return &Event{HookEventName: PreToolUse, ToolName: "Bash"}
}

func TestRunGates(t *testing.T) {
	t.Run("first non-nil decision wins", func(t *testing.T) {
		sandbox(t, []Rule{
			{ID: "gate-a", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Gate: func(context.Context, *Event) *Decision {
					return &Decision{Permission: PermissionDeny, Reason: "first"}
				}},
			{ID: "gate-b", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Gate: func(context.Context, *Event) *Decision {
					return &Decision{Permission: PermissionAllow}
				}},
		})
		if d := RunGates(preEvent()); d == nil || d.Rule != "gate-a" {
			t.Fatalf("first non-nil gate must win, got %+v", d)
		}
	})

	t.Run("a nil defer lets the next gate decide", func(t *testing.T) {
		sandbox(t, []Rule{
			{ID: "gate-defer", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Gate: func(context.Context, *Event) *Decision { return nil }},
			{ID: "gate-decide", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Gate: func(context.Context, *Event) *Decision {
					return &Decision{Permission: PermissionAllow}
				}},
		})
		if d := RunGates(preEvent()); d == nil || d.Rule != "gate-decide" {
			t.Fatalf("a deferring gate must fall through, got %+v", d)
		}
	})

	t.Run("an overrun is abandoned fail-open", func(t *testing.T) {
		sandbox(t, []Rule{
			{ID: "gate-slow", Events: []EventName{PreToolUse}, EnabledByDefault: true, Timeout: 1,
				Gate: func(context.Context, *Event) *Decision {
					time.Sleep(2 * time.Second)
					return &Decision{Permission: PermissionDeny}
				}},
			{ID: "gate-fast", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Gate: func(context.Context, *Event) *Decision {
					return &Decision{Permission: PermissionAllow}
				}},
		})
		if d := RunGates(preEvent()); d == nil || d.Rule != "gate-fast" {
			t.Fatalf("an overrunning gate must be skipped, letting the next decide, got %+v", d)
		}
	})
}

func TestRunRewrites(t *testing.T) {
	t.Run("first non-nil mutation wins", func(t *testing.T) {
		sandbox(t, []Rule{
			{ID: "rw-a", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Rewrite: func(context.Context, *Event) *Mutation {
					return &Mutation{UpdatedInput: map[string]any{"n": 1}}
				}},
			{ID: "rw-b", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Rewrite: func(context.Context, *Event) *Mutation {
					return &Mutation{UpdatedInput: map[string]any{"n": 2}}
				}},
		})
		if m := RunRewrites(preEvent()); m == nil || m.Rule != "rw-a" {
			t.Fatalf("first non-nil rewrite must win, got %+v", m)
		}
	})

	t.Run("a nil defer lets the next rewrite apply", func(t *testing.T) {
		sandbox(t, []Rule{
			{ID: "rw-defer", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Rewrite: func(context.Context, *Event) *Mutation { return nil }},
			{ID: "rw-apply", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Rewrite: func(context.Context, *Event) *Mutation {
					return &Mutation{UpdatedInput: map[string]any{"n": 9}}
				}},
		})
		if m := RunRewrites(preEvent()); m == nil || m.Rule != "rw-apply" {
			t.Fatalf("a deferring rewrite must fall through, got %+v", m)
		}
	})

	t.Run("an overrun is abandoned fail-open", func(t *testing.T) {
		sandbox(t, []Rule{
			{ID: "rw-slow", Events: []EventName{PreToolUse}, EnabledByDefault: true, Timeout: 1,
				Rewrite: func(context.Context, *Event) *Mutation {
					time.Sleep(2 * time.Second)
					return &Mutation{UpdatedInput: map[string]any{"n": 1}}
				}},
			{ID: "rw-fast", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Rewrite: func(context.Context, *Event) *Mutation {
					return &Mutation{UpdatedInput: map[string]any{"n": 2}}
				}},
		})
		if m := RunRewrites(preEvent()); m == nil || m.Rule != "rw-fast" {
			t.Fatalf("an overrunning rewrite must be skipped, got %+v", m)
		}
	})
}

func TestRunAdvice(t *testing.T) {
	t.Run("gathers every rule and skips a nil", func(t *testing.T) {
		sandbox(t, []Rule{
			{ID: "adv-a", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Advise: func(context.Context, *Event) *Advice { return &Advice{Text: "first"} }},
			{ID: "adv-nil", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Advise: func(context.Context, *Event) *Advice { return nil }},
			{ID: "adv-b", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Advise: func(context.Context, *Event) *Advice { return &Advice{Text: "second"} }},
		})
		got := RunAdvice(preEvent())
		if len(got) != 2 || got[0].Rule != "adv-a" || got[1].Rule != "adv-b" {
			t.Fatalf("advice must gather every rule in order, skipping nil, got %+v", got)
		}
	})

	t.Run("an overrun is abandoned fail-open", func(t *testing.T) {
		sandbox(t, []Rule{
			{ID: "adv-slow", Events: []EventName{PreToolUse}, EnabledByDefault: true, Timeout: 1,
				Advise: func(context.Context, *Event) *Advice {
					time.Sleep(2 * time.Second)
					return &Advice{Text: "late"}
				}},
			{ID: "adv-fast", Events: []EventName{PreToolUse}, EnabledByDefault: true,
				Advise: func(context.Context, *Event) *Advice { return &Advice{Text: "ontime"} }},
		})
		got := RunAdvice(preEvent())
		if len(got) != 1 || got[0].Rule != "adv-fast" {
			t.Fatalf("an overrunning advice rule must be dropped, got %+v", got)
		}
	})
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
