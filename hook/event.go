package hook

import "encoding/json"

// MergeInput builds an updatedInput from the tool's arguments plus changes.
// updatedInput replaces the WHOLE object, so hand-building one drops every
// field you forgot — {"file_path": …} on a Write discards `content`.
func (e *Event) MergeInput(changes map[string]any) map[string]any {
	merged := map[string]any{}
	if len(e.ToolInput) > 0 {
		_ = json.Unmarshal(e.ToolInput, &merged)
	}
	for k, v := range changes {
		merged[k] = v
	}
	return merged
}

// Event is the hook payload Claude Code writes to stdin. Fields are the
// union across hook events; ones absent for a given event stay zero.
type Event struct {
	// Raw is the payload as received, so a per-event accessor can reach fields
	// this flat struct does not carry. Populated by DecodeEvent.
	Raw json.RawMessage `json:"-"`

	HookEventName  EventName       `json:"hook_event_name"`
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolResponse   json.RawMessage `json:"tool_response"`

	// UserPromptSubmit and UserPromptExpansion only: the text the user typed,
	// before Claude has seen it. The one place a rule can stop content from
	// ever reaching the API.
	Prompt string `json:"prompt"`

	// MessageDisplay only. Assistant text arrives as a stream of Delta chunks
	// that split at arbitrary offsets — mid-line and even mid-fence-marker — so
	// a rule reassembles them keyed by MessageID until Final.
	MessageID string `json:"message_id"`
	Delta     string `json:"delta"`
	Index     int    `json:"index"`
	Final     bool   `json:"final"`

	// PostToolUseFailure only. There is no tool_response on a failure — Error
	// carries the exit code and the combined output as one string.
	Error       string `json:"error"`
	IsInterrupt bool   `json:"is_interrupt"`

	// Stop only. LastAssistantMessage is the finished message's full text,
	// handed over directly — no transcript parsing needed to inspect a turn.
	LastAssistantMessage string `json:"last_assistant_message"`
	StopHookActive       bool   `json:"stop_hook_active"`
}
