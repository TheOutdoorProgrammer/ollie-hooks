package hooktest_test

import (
	"context"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

// denyRule registers a fixture that always fires, under a unique id so the
// append-only registry never sees a duplicate across this binary's tests.
func denyRule(id string, ev hook.EventName) {
	hook.Register(hook.Rule{
		ID: id, Events: []hook.EventName{ev}, EnabledByDefault: true,
		Check: func(context.Context, *hook.Event) []hook.Finding {
			return []hook.Finding{{Message: "denied"}}
		},
	})
}

func TestAssertDeniedOnPreToolUse(t *testing.T) {
	denyRule("hooktest-deny-pre", hook.PreToolUse)
	hooktest.NoConfig(t)
	hooktest.AssertDenied(t, hooktest.PreToolUse(t, "Bash", map[string]any{"command": "x"}))
}

func TestAssertDeniedOnStop(t *testing.T) {
	denyRule("hooktest-deny-stop", hook.Stop)
	hooktest.NoConfig(t)
	hooktest.AssertDenied(t, hooktest.Stop(t, "done"))
}

func TestNewEventBuilders(t *testing.T) {
	if ev := hooktest.UserPromptSubmit(t, "hello"); ev.HookEventName != hook.UserPromptSubmit || ev.Prompt != "hello" {
		t.Errorf("UserPromptSubmit: %+v", ev)
	}
	if ev := hooktest.Stop(t, "turn"); ev.HookEventName != hook.Stop || ev.LastAssistantMessage != "turn" {
		t.Errorf("Stop: %+v", ev)
	}
	if ev := hooktest.SubagentStop(t, "sub"); ev.HookEventName != hook.SubagentStop || ev.LastAssistantMessage != "sub" {
		t.Errorf("SubagentStop: %+v", ev)
	}
}
