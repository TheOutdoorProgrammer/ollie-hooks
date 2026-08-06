package hook

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
)

// Fire simulates one hook event from the command line: it builds the event from
// the flags, runs the rules with tracing on, and prints the response envelope.
// A rule author sees what fires without triggering a real Claude Code event.
func Fire(args []string, out io.Writer) int {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(out, format+"\n", a...) }
	if len(args) == 0 {
		p("usage: ollie-hooks fire <EventName> [--tool NAME --input JSON ...]")
		return 2
	}
	event := EventName(args[0])
	if !knownEvents[event] {
		p("fire: %q is not a known event", event)
		return 2
	}

	fs := flag.NewFlagSet("fire", flag.ContinueOnError)
	fs.SetOutput(out)
	var (
		tool     = fs.String("tool", "", "tool_name")
		input    = fs.String("input", "", "tool_input as JSON")
		response = fs.String("response", "", "tool_response as JSON")
		prompt   = fs.String("prompt", "", "prompt text")
		errText  = fs.String("error", "", "error text for a failure")
	)
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	payload := map[string]any{"hook_event_name": string(event)}
	setStr(payload, "tool_name", *tool)
	setStr(payload, "prompt", *prompt)
	setStr(payload, "error", *errText)
	if err := setJSON(payload, "tool_input", *input); err != nil {
		p("fire: --input is not valid JSON: %v", err)
		return 2
	}
	if err := setJSON(payload, "tool_response", *response); err != nil {
		p("fire: --response is not valid JSON: %v", err)
		return 2
	}

	// Build through DecodeEvent, not a struct literal, so the raw payload is kept
	// and per-event accessors work the same as under a real event.
	raw, err := json.Marshal(payload)
	if err != nil {
		p("fire: %v", err)
		return 1
	}
	ev, err := DecodeEvent(bytes.NewReader(raw))
	if err != nil {
		p("fire: %v", err)
		return 1
	}

	// Force the trace on so the rules that ran, and why the rest did not, print
	// to stderr alongside the envelope on stdout.
	debugOn = true
	envelope, err := Decide(ev)
	if err != nil {
		p("fire: %v", err)
		return 1
	}
	if envelope == "" {
		p("(allowed with no output)")
		return 0
	}
	p("%s", envelope)
	return 0
}

// setStr adds a string field to the payload only when it is non-empty.
func setStr(payload map[string]any, key, val string) {
	if val != "" {
		payload[key] = val
	}
}

// setJSON adds a raw-JSON field, rejecting a value that does not parse, so a
// typo is caught here instead of silently decoding to nothing downstream.
func setJSON(payload map[string]any, key, val string) error {
	if val == "" {
		return nil
	}
	if !json.Valid([]byte(val)) {
		return fmt.Errorf("invalid JSON")
	}
	payload[key] = json.RawMessage(val)
	return nil
}
