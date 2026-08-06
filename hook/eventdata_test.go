package hook

import (
	"strings"
	"testing"
)

func decode(t *testing.T, payload string) *Event {
	t.Helper()
	ev, err := DecodeEvent(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("decoding %s: %v", payload, err)
	}
	return ev
}

// The collision this whole file exists for: last_assistant_message is Claude's
// text on Stop and the API ERROR STRING on StopFailure. A rule reading the flat
// field would report "API Error: Rate limit reached" as something Claude said.
func TestLastAssistantMessageMeansTwoDifferentThings(t *testing.T) {
	stop := decode(t, `{"hook_event_name":"Stop","last_assistant_message":"Here is the answer."}`)
	fail := decode(t, `{"hook_event_name":"StopFailure","error":"rate_limit",`+
		`"last_assistant_message":"API Error: Rate limit reached"}`)

	got, ok := stop.Stop()
	if !ok || got.LastAssistantMessage != "Here is the answer." {
		t.Errorf("Stop should carry Claude's text, got %+v ok=%v", got, ok)
	}
	// Asking a StopFailure for Claude's speech must refuse, not return the error.
	if _, ok := fail.Stop(); ok {
		t.Error("StopFailure must not answer Stop() — that is the mis-read")
	}

	f, ok := fail.StopFailure()
	if !ok {
		t.Fatal("StopFailure should decode its own event")
	}
	if f.ErrorType != "rate_limit" {
		t.Errorf("error is an enum here, got %q", f.ErrorType)
	}
	if !strings.Contains(f.APIErrorText, "Rate limit") {
		t.Errorf("the API error text should be reachable, got %q", f.APIErrorText)
	}
}

// source is three different enums on three events. An accessor keyed to the
// wrong one must decline rather than hand back a value from another vocabulary.
func TestSourceIsScopedToItsEvent(t *testing.T) {
	start := decode(t, `{"hook_event_name":"SessionStart","source":"compact"}`)
	cfg := decode(t, `{"hook_event_name":"ConfigChange","source":"user_settings"}`)

	if d, ok := start.SessionStart(); !ok || d.Source != "compact" {
		t.Errorf("SessionStart source = %+v ok=%v", d, ok)
	}
	if _, ok := start.ConfigChange(); ok {
		t.Error("a SessionStart must not answer ConfigChange()")
	}
	if d, ok := cfg.ConfigChange(); !ok || d.Source != "user_settings" {
		t.Errorf("ConfigChange source = %+v ok=%v", d, ok)
	}
	if _, ok := cfg.SessionStart(); ok {
		t.Error("a ConfigChange must not answer SessionStart()")
	}
}

// Here file_path is the watched file rather than a tool target, and the change
// kind arrives under a key called "event".
func TestFileChangedCarriesTheWatchedFile(t *testing.T) {
	ev := decode(t, `{"hook_event_name":"FileChanged","file_path":"/w/x.go","event":"change"}`)
	d, ok := ev.FileChanged()
	if !ok || d.FilePath != "/w/x.go" || d.Change != "change" {
		t.Errorf("FileChanged = %+v ok=%v", d, ok)
	}
}

func TestPreCompactTrigger(t *testing.T) {
	ev := decode(t, `{"hook_event_name":"PreCompact","trigger":"auto"}`)
	if d, ok := ev.PreCompact(); !ok || d.Trigger != "auto" {
		t.Errorf("PreCompact = %+v ok=%v", d, ok)
	}
}

func TestNotificationMessage(t *testing.T) {
	ev := decode(t, `{"hook_event_name":"Notification","message":"needs your attention"}`)
	if d, ok := ev.Notification(); !ok || d.Message != "needs your attention" {
		t.Errorf("Notification = %+v ok=%v", d, ok)
	}
}

// SubagentStop carries the same shape as Stop, so it shares the accessor.
func TestSubagentStopSharesTheStopShape(t *testing.T) {
	ev := decode(t, `{"hook_event_name":"SubagentStop","last_assistant_message":"done"}`)
	if d, ok := ev.Stop(); !ok || d.LastAssistantMessage != "done" {
		t.Errorf("SubagentStop = %+v ok=%v", d, ok)
	}
}

// An Event built by hand rather than decoded has no raw payload, so accessors
// must decline instead of returning confident zero values.
func TestAccessorsDeclineWithoutRawPayload(t *testing.T) {
	ev := &Event{HookEventName: Stop}
	if _, ok := ev.Stop(); ok {
		t.Error("no raw payload means nothing to decode")
	}
}

func TestDecodeEventKeepsTheRawPayload(t *testing.T) {
	ev := decode(t, `{"hook_event_name":"PostToolUse","tool_name":"Bash"}`)
	if len(ev.Raw) == 0 {
		t.Fatal("raw payload must be retained")
	}
	if ev.HookEventName != PostToolUse || ev.ToolName != "Bash" {
		t.Errorf("flat fields must still populate, got %+v", ev)
	}
}

func TestCommandAccessor(t *testing.T) {
	ev := &Event{ToolName: "Bash", ToolInput: []byte(`{"command":"ls -la"}`)}
	if got, ok := ev.Command(); !ok || got != "ls -la" {
		t.Errorf("Command = %q ok=%v", got, ok)
	}
	for _, in := range []string{`{}`, `{"command":""}`, `not json`} {
		if _, ok := (&Event{ToolInput: []byte(in)}).Command(); ok {
			t.Errorf("%s should yield no command", in)
		}
	}
}

// A failure carries no tool_response at all, so a rule reading only that field
// goes silent on exactly the failures it most wants to see.
func TestBashResultReadsTheFailureField(t *testing.T) {
	ok_ := &Event{
		HookEventName: PostToolUse,
		ToolResponse:  []byte(`{"stdout":"out","stderr":"err"}`),
	}
	if got, ok := ok_.BashResult(); !ok || got != "out\nerr" {
		t.Errorf("success = %q ok=%v", got, ok)
	}

	failed := &Event{HookEventName: PostToolUseFailure, Error: "Exit code 1\nboom"}
	got, ok := failed.BashResult()
	if !ok || !strings.Contains(got, "boom") {
		t.Errorf("failure output must come from Error, got %q ok=%v", got, ok)
	}

	empty := &Event{HookEventName: PostToolUseFailure}
	if _, ok := empty.BashResult(); ok {
		t.Error("an empty failure has nothing to report")
	}
}
