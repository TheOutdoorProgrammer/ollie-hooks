package hook

// Verb is one way a rule can act on an event. An event accepts only some of
// them, and the mismatches are silent: a Check rule on an event whose envelope
// has no deny path registers happily and does nothing forever.
type Verb string

const (
	VerbCheck   Verb = "Check"
	VerbRewrite Verb = "Rewrite"
	VerbGate    Verb = "Gate"
	VerbDisplay Verb = "Display"
	VerbAdvise  Verb = "Advise"
)

// EventName is a Claude Code hook event. Typed for discoverability and to keep
// a plain string variable out of a Rule; typos in literals are caught at
// registration instead, since Go converts untyped constants freely.
type EventName string

// The hook events Claude Code can send. eventCaps records what each accepts;
// an event listed here but absent there is simply unconstrained.
const (
	PreToolUse         EventName = "PreToolUse"
	PostToolUse        EventName = "PostToolUse"
	PostToolUseFailure EventName = "PostToolUseFailure"
	PostToolBatch      EventName = "PostToolBatch"
	PermissionRequest  EventName = "PermissionRequest"
	PermissionDenied   EventName = "PermissionDenied"

	UserPromptSubmit    EventName = "UserPromptSubmit"
	UserPromptExpansion EventName = "UserPromptExpansion"
	MessageDisplay      EventName = "MessageDisplay"
	Stop                EventName = "Stop"
	StopFailure         EventName = "StopFailure"

	SessionStart  EventName = "SessionStart"
	SessionEnd    EventName = "SessionEnd"
	Setup         EventName = "Setup"
	SubagentStart EventName = "SubagentStart"
	SubagentStop  EventName = "SubagentStop"

	TaskCreated   EventName = "TaskCreated"
	TaskCompleted EventName = "TaskCompleted"
	TeammateIdle  EventName = "TeammateIdle"

	ConfigChange       EventName = "ConfigChange"
	CwdChanged         EventName = "CwdChanged"
	DirectoryAdded     EventName = "DirectoryAdded"
	FileChanged        EventName = "FileChanged"
	InstructionsLoaded EventName = "InstructionsLoaded"

	PreCompact     EventName = "PreCompact"
	PostCompact    EventName = "PostCompact"
	WorktreeCreate EventName = "WorktreeCreate"
	WorktreeRemove EventName = "WorktreeRemove"

	Notification      EventName = "Notification"
	Elicitation       EventName = "Elicitation"
	ElicitationResult EventName = "ElicitationResult"
)

// knownEvents is every event Claude Code sends. Go converts untyped string
// constants freely, so the EventName type alone would admit a typo — and a
// typo.d event fires never, with nothing to diagnose.
//
// Stricter than eventCaps on purpose: whether a NAME is real and what it can
// express are different questions, and we know the full list of names.
var knownEvents = map[EventName]bool{
	PreToolUse: true, PostToolUse: true, PostToolUseFailure: true,
	PostToolBatch: true, PermissionRequest: true, PermissionDenied: true,
	UserPromptSubmit: true, UserPromptExpansion: true, MessageDisplay: true,
	Stop: true, StopFailure: true,
	SessionStart: true, SessionEnd: true, Setup: true,
	SubagentStart: true, SubagentStop: true,
	TaskCreated: true, TaskCompleted: true, TeammateIdle: true,
	ConfigChange: true, CwdChanged: true, DirectoryAdded: true,
	FileChanged: true, InstructionsLoaded: true,
	PreCompact: true, PostCompact: true,
	WorktreeCreate: true, WorktreeRemove: true,
	Notification: true, Elicitation: true, ElicitationResult: true,
}

// matcherKind is what an event's matcher is compared against. Not cosmetic:
// for FileChanged the matcher IS the watch list, so "*" would register a watch
// on a file literally named "*".
type matcherKind string

const (
	matchNone    matcherKind = ""           // matcher rejected; entry takes ""
	matchTool    matcherKind = "tool_name"  //
	matchSource  matcherKind = "source"     //
	matchTrigger matcherKind = "trigger"    //
	matchCommand matcherKind = "command"    //
	matchError   matcherKind = "error_type" //
	matchAgent   matcherKind = "agent_type" //
	matchFile    matcherKind = "file_names" //
)

// eventCap describes what one hook event can express. TimeoutMax is a real
// ceiling the platform refuses to exceed — only SessionEnd documents one. The
// per-event DEFAULTS (10s, 30s, 600s) are not ceilings: a longer entry raises them.
type eventCap struct {
	Matcher    matcherKind
	Verbs      []Verb
	TimeoutMax int
}

// eventCaps is the per-event contract. An event absent from this map is
// treated as permissive: unknown is not the same as forbidden, and the whole
// framework fails open, so a new Claude Code event must never be a hard error.
var eventCaps = map[EventName]eventCap{
	// Tool loop.
	PreToolUse: {
		Matcher: matchTool,
		Verbs:   []Verb{VerbCheck, VerbRewrite, VerbGate, VerbAdvise},
	},
	PostToolUse: {
		Matcher: matchTool,
		Verbs:   []Verb{VerbCheck, VerbRewrite, VerbDisplay, VerbAdvise},
	},
	PostToolUseFailure: {
		Matcher: matchTool,
		Verbs:   []Verb{VerbCheck, VerbDisplay, VerbAdvise},
	},
	PermissionRequest: {
		Matcher: matchTool,
		// Gate only. This event carries updatedInput inside its decision object,
		// valid solely alongside an allow, so a bare Rewrite has no envelope to
		// emit here — Decision.UpdatedInput is how you change the input.
		Verbs: []Verb{VerbGate},
	},
	PostToolBatch: {
		Matcher: matchNone,
		Verbs:   []Verb{VerbCheck, VerbAdvise},
	},

	// Turn.
	UserPromptSubmit: {
		Matcher: matchNone,
		Verbs:   []Verb{VerbCheck, VerbAdvise},
	},
	UserPromptExpansion: {
		Matcher: matchCommand,
		Verbs:   []Verb{VerbCheck, VerbAdvise},
	},
	MessageDisplay: {
		Matcher: matchNone,
		Verbs:   []Verb{VerbDisplay},
	},
	Stop: {
		Matcher: matchNone,
		Verbs:   []Verb{VerbCheck, VerbDisplay, VerbAdvise},
	},

	// Session.
	SessionStart: {
		Matcher: matchSource,
		Verbs:   []Verb{VerbAdvise},
	},
	SessionEnd: {
		Matcher:    matchNone,
		TimeoutMax: 60,
	},
	SubagentStart: {
		Matcher: matchAgent,
		Verbs:   []Verb{VerbAdvise},
	},
	SubagentStop: {
		Matcher: matchAgent,
		Verbs:   []Verb{VerbCheck, VerbAdvise},
	},

	// Config and filesystem.
	ConfigChange: {
		Matcher: matchSource,
		Verbs:   []Verb{VerbCheck},
	},
	CwdChanged: {Matcher: matchNone},
	FileChanged: {
		Matcher: matchFile,
	},

	// Compaction.
	PreCompact: {
		Matcher: matchTrigger,
		Verbs:   []Verb{VerbCheck},
	},
	PostCompact: {Matcher: matchNone},

	// MCP elicitation.
	Elicitation: {
		Verbs: []Verb{VerbGate},
	},
	// Same envelope as Elicitation, fired after the user answers: a Gate here
	// overrides what they chose.
	ElicitationResult: {
		Verbs: []Verb{VerbGate},
	},

	// Output-only.
	Notification: {
		Verbs: []Verb{VerbDisplay},
	},

	// Known but unactionable: Claude Code ignores hook output here, or we don't
	// act on it yet. No Verbs on purpose — registration then panics loudly
	// instead of silently no-opping at fire time. Absent stays permissive.
	StopFailure:        {Matcher: matchError},
	PermissionDenied:   {Matcher: matchTool},
	InstructionsLoaded: {},
	Setup:              {},
	TaskCreated:        {Matcher: matchAgent},
	TaskCompleted:      {Matcher: matchAgent},
	TeammateIdle:       {Matcher: matchAgent},
	DirectoryAdded:     {},
	WorktreeCreate:     {},
	WorktreeRemove:     {},
}

// eventActionable reports whether a rule can register a behaviour on an event.
// A known event with no Verbs is inert; an unknown event is permissive, since
// the framework fails open on new events.
func eventActionable(event EventName) bool {
	cap, known := eventCaps[event]
	return !known || len(cap.Verbs) > 0
}

// eventAllows reports whether an event can express a verb. Unknown events
// allow everything: the map is a guard against known-impossible pairings, not
// an allowlist of events we happen to have documented.
func eventAllows(event EventName, verb Verb) bool {
	cap, known := eventCaps[event]
	if !known {
		return true
	}
	for _, v := range cap.Verbs {
		if v == verb {
			return true
		}
	}
	return false
}

// eventMatcher returns the settings.json matcher for an event. Matcherless
// events take "" rather than the "*" tool events use.
func eventMatcher(event EventName) string {
	if cap, known := eventCaps[event]; known && cap.Matcher == matchNone {
		return ""
	}
	return "*"
}

// wiringTimeout keeps Claude Code from ever killing us: every rule is already
// bounded in-process, and a killed process returns nothing while abandoning one
// rule still returns what the others found. A Gate on human review runs hours.
const wiringTimeout = 86400

// eventTimeout is the settings-entry budget for an event, respecting the one
// documented hard ceiling.
func eventTimeout(event EventName) int {
	if cap, known := eventCaps[event]; known && cap.TimeoutMax > 0 {
		return cap.TimeoutMax
	}
	return wiringTimeout
}
