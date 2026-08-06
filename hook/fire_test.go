package hook

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestFireRejectsUnknownEvent(t *testing.T) {
	var out bytes.Buffer
	if code := Fire([]string{"NotAnEvent"}, &out); code != 2 {
		t.Errorf("unknown event should exit 2, got %d", code)
	}
	if !strings.Contains(out.String(), "not a known event") {
		t.Errorf("expected a clear message, got %q", out.String())
	}
}

func TestFireRejectsInvalidInputJSON(t *testing.T) {
	var out bytes.Buffer
	if code := Fire([]string{"PreToolUse", "--tool", "Bash", "--input", "{not json"}, &out); code != 2 {
		t.Errorf("invalid --input should exit 2, got %d", code)
	}
}

// A registered rule that fires must reach fire's output as a deny envelope, so
// the test proves fire drives the whole Decide path, not just the rule.
func TestFireShowsABlock(t *testing.T) {
	writeUserConfig(t, "")
	t.Cleanup(SwapRegistry([]Rule{{
		ID: "fire-demo", Events: []EventName{PreToolUse}, EnabledByDefault: true,
		Check: func(context.Context, *Event) []Finding {
			return []Finding{{Message: "blocked by fire-demo"}}
		},
	}}))
	saved := debugOn
	t.Cleanup(func() { debugOn = saved })

	var out bytes.Buffer
	code := Fire([]string{"PreToolUse", "--tool", "Bash", "--input", `{"command":"x"}`}, &out)
	if code != 0 {
		t.Errorf("fire itself succeeds even when the event is denied, got exit %d", code)
	}
	if !strings.Contains(out.String(), "permissionDecision") ||
		!strings.Contains(out.String(), "blocked by fire-demo") {
		t.Errorf("expected a deny envelope, got %q", out.String())
	}
}
