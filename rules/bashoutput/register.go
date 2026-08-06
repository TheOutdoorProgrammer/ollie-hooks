package bashoutput

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "bash-output"

// Register adds the rule. One rule, two events: a non-zero exit lands on
// PostToolUseFailure rather than PostToolUse, but echoing the output is one
// feature and should not be two things for the user to configure.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Re-print a collapsed Bash result in full, so it needs no ctrl+o",
		Events:      []hook.EventName{hook.PostToolUse, hook.PostToolUseFailure},
		Tools:       []string{"Bash"},
		Display:     displayBashOutput,
	})
}
