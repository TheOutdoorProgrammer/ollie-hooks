package hook

import (
	"encoding/json"
	"io"
)

// DecodeEvent reads a hook payload, keeping the original bytes so per-event
// accessors can reach fields the flat struct does not carry.
func DecodeEvent(r io.Reader) (*Event, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var ev Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil, err
	}
	ev.Raw = raw
	return &ev, nil
}

// decodeAs unmarshals the raw payload into v, but only when the event matches.
// The check is the point: several keys carry different meanings per event, and
// a silent mis-read is worse than no read at all.
func decodeAs[T any](e *Event, want EventName, v *T) bool {
	if e.HookEventName != want || len(e.Raw) == 0 {
		return false
	}
	return json.Unmarshal(e.Raw, v) == nil
}

// StopData is what a finished turn carries.
type StopData struct {
	// LastAssistantMessage is Claude's finished text. Read it from here rather
	// than the transcript, which is written asynchronously and lags.
	LastAssistantMessage string `json:"last_assistant_message"`
	StopHookActive       bool   `json:"stop_hook_active"`
}

// Stop returns the finished-turn payload, or false on any other event.
func (e *Event) Stop() (StopData, bool) {
	var d StopData
	ok := decodeAs(e, Stop, &d) || decodeAs(e, SubagentStop, &d)
	return d, ok
}

// StopFailureData is a turn that ended in an API error. The trap it exists for:
// here last_assistant_message is the API ERROR STRING, not anything Claude
// said, so reading it as speech reports the error as a statement by the model.
type StopFailureData struct {
	ErrorType string `json:"error"`
	// APIErrorText is last_assistant_message on this event — deliberately named
	// for what it holds rather than which key it came from.
	APIErrorText string `json:"last_assistant_message"`
}

// StopFailure returns the failed-turn payload, or false on any other event.
func (e *Event) StopFailure() (StopFailureData, bool) {
	var d StopFailureData
	return d, decodeAs(e, StopFailure, &d)
}

// SessionStartData says how a session began: startup, resume, clear, compact
// or fork. "compact" is the one that matters — it is where context lost to a
// compaction can be put back.
type SessionStartData struct {
	Source string `json:"source"`
	Model  string `json:"model"`
}

// SessionStart returns the session-start payload, or false on any other event.
func (e *Event) SessionStart() (SessionStartData, bool) {
	var d SessionStartData
	return d, decodeAs(e, SessionStart, &d)
}

// ConfigChangeData describes a settings file about to change. Its Source is a
// different enum from SessionStart's despite sharing the key name.
type ConfigChangeData struct {
	Source   string `json:"source"`
	FilePath string `json:"file_path"`
}

// ConfigChange returns the settings-change payload, or false otherwise.
func (e *Event) ConfigChange() (ConfigChangeData, bool) {
	var d ConfigChangeData
	return d, decodeAs(e, ConfigChange, &d)
}

// CompactData says whether a compaction was asked for or automatic.
type CompactData struct {
	Trigger            string `json:"trigger"`
	CustomInstructions string `json:"custom_instructions"`
}

// PreCompact returns the pre-compaction payload, or false otherwise.
func (e *Event) PreCompact() (CompactData, bool) {
	var d CompactData
	return d, decodeAs(e, PreCompact, &d)
}

// NotificationData is a message meant for the user, not the model.
type NotificationData struct {
	Message string `json:"message"`
}

// Notification returns the notification payload, or false otherwise.
func (e *Event) Notification() (NotificationData, bool) {
	var d NotificationData
	return d, decodeAs(e, Notification, &d)
}

// FileChangedData describes a watched file. Its FilePath is the watched file,
// not a tool's target — reading tool_input here would find nothing.
type FileChangedData struct {
	FilePath string `json:"file_path"`
	Change   string `json:"event"`
}

// FileChanged returns the watched-file payload, or false otherwise.
func (e *Event) FileChanged() (FileChangedData, bool) {
	var d FileChangedData
	return d, decodeAs(e, FileChanged, &d)
}

// PermissionDeniedData is the auto-mode classifier's refusal.
type PermissionDeniedData struct {
	Reason string `json:"reason"`
}

// PermissionDenied returns the refusal payload, or false otherwise.
func (e *Event) PermissionDenied() (PermissionDeniedData, bool) {
	var d PermissionDeniedData
	return d, decodeAs(e, PermissionDenied, &d)
}

// Command is a Bash tool's command line. Bash is the most-guarded tool in
// practice, and every rule touching it was decoding tool_input by hand.
func (e *Event) Command() (string, bool) {
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(e.ToolInput, &in); err != nil || in.Command == "" {
		return "", false
	}
	return in.Command, true
}

// BashResult is a Bash tool's output. On PostToolUseFailure there is no
// tool_response at all — the output arrives in `error` alongside the exit code —
// so a rule reading only tool_response goes silent on exactly the failures it
// most wants to see.
func (e *Event) BashResult() (string, bool) {
	if e.HookEventName == PostToolUseFailure {
		if e.Error == "" {
			return "", false
		}
		return e.Error, true
	}
	var resp struct {
		Stdout string `json:"stdout"`
		Stderr string `json:"stderr"`
	}
	if err := json.Unmarshal(e.ToolResponse, &resp); err != nil {
		return "", false
	}
	out := resp.Stdout
	if resp.Stderr != "" {
		if out != "" {
			out += "\n"
		}
		out += resp.Stderr
	}
	return out, out != ""
}
