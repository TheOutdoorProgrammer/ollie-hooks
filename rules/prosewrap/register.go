package prosewrap

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "prose-wrap"

// Register adds the rule to the registry.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Flag markdown prose a change hard-wrapped mid-sentence (advisory) — one sentence per line",
		Events:      []hook.EventName{hook.PostToolUse},
		Tools:       []string{"Write", "Edit", "MultiEdit"},
		// A house style, not a correctness rule — it should read as a suggestion.
		DefaultSeverity: hook.SeverityAdvisory,
		Check:           checkProseWrap,
	})
}
