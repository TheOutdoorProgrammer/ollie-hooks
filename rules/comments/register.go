package comments

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "comments"

// Register adds the rule to the registry.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Flag verbose or agent-memo comments a change touched (advisory) — justify, slim, or remove",
		Events:      []hook.EventName{hook.PostToolUse},
		Tools:       []string{"Write", "Edit", "MultiEdit"},
		// Comment policy is opinionated; blocking on it is too strong a default.
		DefaultSeverity: hook.SeverityAdvisory,
		Check:           checkComments,
	})
}
