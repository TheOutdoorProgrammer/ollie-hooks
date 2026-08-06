package hook

import (
	"context"
	"testing"
)

func TestEventAllowsKnownPairings(t *testing.T) {
	cases := []struct {
		event EventName
		verb  Verb
		want  bool
	}{
		{"PreToolUse", VerbGate, true},
		{"PreToolUse", VerbDisplay, false},
		{"MessageDisplay", VerbDisplay, true},
		{"MessageDisplay", VerbCheck, false},
		{"Stop", VerbCheck, true},
		{"PostToolUse", VerbRewrite, true},
		{"SessionStart", VerbCheck, false},
	}
	for _, c := range cases {
		if got := eventAllows(c.event, c.verb); got != c.want {
			t.Errorf("eventAllows(%q, %q) = %v, want %v", c.event, c.verb, got, c.want)
		}
	}
}

// An event we have no data for must stay usable: the table guards against
// known-impossible pairings, it is not an allowlist of documented events.
func TestEventAllowsUnknownEvent(t *testing.T) {
	for _, v := range []Verb{VerbCheck, VerbRewrite, VerbGate, VerbDisplay} {
		if !eventAllows("SomeFutureEvent", v) {
			t.Errorf("unknown event must permit %q", v)
		}
	}
}

func TestRegisterPanicsOnImpossibleVerb(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a Check rule on MessageDisplay must panic, not register and never run")
		}
	}()
	Register(Rule{
		ID:     "test-impossible",
		Events: []EventName{MessageDisplay},
		Check:  func(context.Context, *Event) []Finding { return nil },
	})
}

// The settings timeout exists so Claude Code never kills us: a killed process
// returns nothing, abandoning one slow rule still returns the rest.
func TestEventTimeoutIsLongExceptWhereCapped(t *testing.T) {
	for _, ev := range []EventName{PreToolUse, PostToolUse, UserPromptSubmit, MessageDisplay} {
		if got := eventTimeout(ev); got != wiringTimeout {
			t.Errorf("%s should ask for the long budget, got %d", ev, got)
		}
	}
	// MessageDisplay's 10s and UserPromptSubmit's 30s are raisable DEFAULTS, so
	// asking for more is legitimate. SessionEnd is the one documented ceiling.
	if got := eventTimeout(SessionEnd); got != 60 {
		t.Errorf("SessionEnd is capped at 60s by the platform, got %d", got)
	}
}

func TestEventMatcher(t *testing.T) {
	if got := eventMatcher("PreToolUse"); got != "*" {
		t.Errorf("tool events take a wildcard matcher, got %q", got)
	}
	if got := eventMatcher("MessageDisplay"); got != "" {
		t.Errorf("matcherless events must take an empty matcher, got %q", got)
	}
	if got := eventMatcher("CwdChanged"); got != "" {
		t.Errorf("CwdChanged is matcherless, got %q", got)
	}
}

// Go converts untyped string constants freely, so the EventName type alone
// admits a typo — and a typo'd event fires never, with nothing to diagnose.
func TestRegisterRejectsAnUnknownEvent(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a misspelled event must panic, not register and never fire")
		}
	}()
	Register(Rule{
		ID:     "typo-demo",
		Events: []EventName{"PostToolUsage"},
		Check:  func(context.Context, *Event) []Finding { return nil },
	})
}

// Every event in eventCaps must be a real one, or the table is describing
// something that will never arrive.
func TestEveryCappedEventIsKnown(t *testing.T) {
	for ev := range eventCaps {
		if !knownEvents[ev] {
			t.Errorf("eventCaps describes %q, which is not a known event", ev)
		}
	}
}

// The other half of that contract: a known event with no cap silently accepts
// any verb and then fails at fire time. Every known event must be capped so the
// failure lands loudly at registration instead.
func TestEveryKnownEventIsCapped(t *testing.T) {
	for ev := range knownEvents {
		if _, ok := eventCaps[ev]; !ok {
			t.Errorf("known event %q has no eventCaps entry, so it is silently permissive", ev)
		}
	}
}

// Setup delivers no actionable hook output, so a rule listing it must panic at
// registration rather than register and quietly never fire.
func TestRegisterPanicsOnInertEvent(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a rule on an inert event must panic, not register and never run")
		}
	}()
	Register(Rule{
		ID:     "test-inert",
		Events: []EventName{Setup},
		Check:  func(context.Context, *Event) []Finding { return nil },
	})
}
