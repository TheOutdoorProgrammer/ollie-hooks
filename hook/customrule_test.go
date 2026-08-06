package hook

import (
	"context"
	"strings"
	"testing"
	"time"
)

// countingTransport records how many times a plugin was actually called.
type countingTransport struct {
	reply string
	calls int
	asked []string
}

func (t *countingTransport) call(_ context.Context, payload []byte) ([]byte, error) {
	t.calls++
	t.asked = append(t.asked, string(payload))
	return []byte(t.reply), nil
}

func freshBroker(t *testing.T) {
	t.Helper()
	saved := broker
	savedRegistry := registry
	t.Cleanup(func() { broker, registry = saved, savedRegistry })
	registry = nil
	broker = &pluginBroker{
		entries: map[string]*brokerEntry{},
		members: map[string][]string{},
	}
}

// Several rules served by one endpoint must cost one call, not one each —
// otherwise a plugin with five roles spawns five processes per event.
func TestOneEndpointIsCalledOncePerEvent(t *testing.T) {
	freshBroker(t)
	tr := &countingTransport{reply: `{"rules":{
		"a":{"findings":[{"message":"from a"}]},
		"b":{"advice":"from b"}}}`}

	const key = "shared-endpoint"
	joinPlugin(t, key, "a")
	joinPlugin(t, key, "b")
	ev := &Event{HookEventName: PreToolUse}

	first := broker.response(context.Background(), key, tr, ev, "a")
	second := broker.response(context.Background(), key, tr, ev, "b")

	if tr.calls != 1 {
		t.Errorf("two rules on one endpoint should cost one call, got %d", tr.calls)
	}
	if first == nil || len(first.Findings) != 1 || first.Findings[0].Message != "from a" {
		t.Errorf("rule a got the wrong slice: %+v", first)
	}
	if second == nil || second.Advice != "from b" {
		t.Errorf("rule b got the wrong slice: %+v", second)
	}
}

// The plugin is told which roles are being asked, so it can answer for all.
func TestRequestNamesEveryRule(t *testing.T) {
	freshBroker(t)
	tr := &countingTransport{reply: `{}`}
	joinPlugin(t, "k", "one")
	joinPlugin(t, "k", "two")

	broker.response(context.Background(), "k", tr, &Event{HookEventName: PreToolUse}, "one")
	if len(tr.asked) != 1 {
		t.Fatalf("want one request, got %d", len(tr.asked))
	}
	for _, want := range []string{`"one"`, `"two"`} {
		if !strings.Contains(tr.asked[0], want) {
			t.Errorf("request should name %s: %s", want, tr.asked[0])
		}
	}
}

// A single-role plugin answers with the bare response, no map needed.
func TestBareReplyAppliesToTheAskingRule(t *testing.T) {
	freshBroker(t)
	tr := &countingTransport{reply: `{"findings":[{"message":"plain"}]}`}
	broker.join("solo", "only")

	got := broker.response(context.Background(), "solo", tr, &Event{HookEventName: PreToolUse}, "only")
	if got == nil || len(got.Findings) != 1 || got.Findings[0].Message != "plain" {
		t.Errorf("bare reply should apply to the asking rule, got %+v", got)
	}
}

// The declared verb is a capability: a plugin returning a decision while
// registered to advise must be ignored, not obeyed.
func TestVerbDecidesWhichFieldIsRead(t *testing.T) {
	r, err := customRule("advisor", CustomRule{
		StartupCmd: "true", Verb: VerbAdvise,
		Events: []EventName{PreToolUse}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Advise == nil {
		t.Fatal("an advise rule should occupy the Advise slot")
	}
	if r.Check != nil || r.Gate != nil || r.Rewrite != nil || r.Display != nil {
		t.Error("declaring advise must not grant any other verb")
	}
}

func TestCustomRuleRejectsBadEntries(t *testing.T) {
	cases := map[string]CustomRule{
		"no endpoint": {Verb: VerbCheck, Events: []EventName{PreToolUse}},
		"two endpoints": {StartupCmd: "true", ServerURL: "http://x",
			Verb: VerbCheck, Events: []EventName{PreToolUse}},
		"no events":  {StartupCmd: "true", Verb: VerbCheck},
		"bogus verb": {StartupCmd: "true", Verb: "destroy", Events: []EventName{PreToolUse}},
		"empty verb": {StartupCmd: "true", Events: []EventName{PreToolUse}},
	}
	for name, c := range cases {
		if _, err := customRule("x", c); err == nil {
			t.Errorf("%s should be rejected", name)
		}
	}
}

// A plugin that dies must not take the session with it. `false` exists but
// exits non-zero, which is the runtime failure this guards; a command that is
// missing entirely is rejected earlier, by customRule.
func TestAPluginFailureIsSurvivable(t *testing.T) {
	skipOnWindows(t)
	freshBroker(t)
	r, err := customRule("crashy", CustomRule{
		StartupCmd: "false", Verb: VerbCheck,
		Events: []EventName{PreToolUse}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Check(context.Background(), &Event{HookEventName: PreToolUse}); got != nil {
		t.Errorf("a dead plugin should yield nothing, got %+v", got)
	}
}

// A startup_cmd that does not exist is caught when the rule is built, not on
// first fire. Plugins fail open, so without this the only symptom of a typo'd
// path is a rule that never does anything.
func TestCustomRuleRejectsAMissingCommand(t *testing.T) {
	_, err := customRule("gone", CustomRule{
		StartupCmd: "definitely-not-a-real-binary-xyzzy", Verb: VerbCheck,
		Events: []EventName{PreToolUse}, Enabled: true,
	})
	if err == nil {
		t.Fatal("a missing startup_cmd must be reported, not deferred to a silent runtime failure")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should say what is missing, got: %v", err)
	}
}

// joinPlugin wires a stub rule into both the registry and the broker, the way
// RegisterCustomRules does. Joining without registering would leave the rule
// invisible to the per-event filtering.
func joinPlugin(t *testing.T, key, id string, events ...EventName) {
	t.Helper()
	if len(events) == 0 {
		events = []EventName{PreToolUse}
	}
	registry = append(registry, Rule{
		ID: id, Events: events,
		Check: func(context.Context, *Event) []Finding { return nil },
	})
	broker.join(key, id)
}

// The request names the roles being asked for THIS event, not every role the
// endpoint serves. A plugin handling a PreToolUse check and a PostToolUse
// rewrite should not be told about both on every event.
func TestRequestNamesOnlyTheRulesThisEventAsksFor(t *testing.T) {
	freshBroker(t)
	tr := &countingTransport{reply: `{}`}
	joinPlugin(t, "k", "pre-rule", PreToolUse)
	joinPlugin(t, "k", "post-rule", PostToolUse)

	broker.response(context.Background(), "k", tr, &Event{HookEventName: PreToolUse}, "pre-rule")

	if len(tr.asked) != 1 {
		t.Fatalf("want one request, got %d", len(tr.asked))
	}
	if !strings.Contains(tr.asked[0], `"pre-rule"`) {
		t.Errorf("request should name the rule being asked: %s", tr.asked[0])
	}
	if strings.Contains(tr.asked[0], `"post-rule"`) {
		t.Errorf("request named a rule this event never fires: %s", tr.asked[0])
	}
}

// The memo is per event. Reusing it across events would replay the first
// event's answers, which is what an embedder calling Decide twice would hit.
func TestBrokerForgetsBetweenEvents(t *testing.T) {
	freshBroker(t)
	tr := &countingTransport{reply: `{}`}
	joinPlugin(t, "k", "r")
	ev := &Event{HookEventName: PreToolUse}

	broker.response(context.Background(), "k", tr, ev, "r")
	broker.response(context.Background(), "k", tr, ev, "r")
	if tr.calls != 1 {
		t.Fatalf("same event should cost one call, got %d", tr.calls)
	}

	broker.reset()
	broker.response(context.Background(), "k", tr, ev, "r")
	if tr.calls != 2 {
		t.Errorf("a new event must call again, got %d call(s)", tr.calls)
	}
}

// A rule with a short timeout must not cut short the shared call that a
// patient sibling is also waiting on.
func TestSharedBudgetTakesTheLongestTimeout(t *testing.T) {
	writeUserConfig(t, `
[rules.quick]
timeout = 1

[rules.patient]
timeout = 30
`)
	if got := sharedBudget([]string{"quick", "patient"}); got != 30*time.Second {
		t.Errorf("shared budget = %v, want the most generous member's 30s", got)
	}
}
