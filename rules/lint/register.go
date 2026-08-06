package lint

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "lint"

// Register adds the rule to the registry. PostToolUse and Advise, not Check:
// the edit has already landed, so the output is context for the model rather
// than a violation — a block envelope here renders as a hook error.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "After an edit, run the file's linter and report what it found (never blocks the edit)",
		Events:      []hook.EventName{hook.PostToolUse},
		Tools:       []string{"Write", "Edit", "MultiEdit"},
		Advise:      adviseLint,
	})
}
