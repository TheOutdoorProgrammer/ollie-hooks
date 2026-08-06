package hook

import (
	"context"
	"strings"
	"testing"
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
	t.Cleanup(func() { broker = saved })
	broker = &pluginBroker{
		byKey:   map[string]pluginReply{},
		failed:  map[string]bool{},
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
	broker.join(key, "a")
	broker.join(key, "b")
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
	broker.join("k", "one")
	broker.join("k", "two")

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

// A plugin that dies must not take the session with it.
func TestAPluginFailureIsSurvivable(t *testing.T) {
	freshBroker(t)
	r, err := customRule("crashy", CustomRule{
		StartupCmd: "definitely-not-a-real-binary-xyzzy", Verb: VerbCheck,
		Events: []EventName{PreToolUse}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Check(context.Background(), &Event{HookEventName: PreToolUse}); got != nil {
		t.Errorf("a dead plugin should yield nothing, got %+v", got)
	}
}
