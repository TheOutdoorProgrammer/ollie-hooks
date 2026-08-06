package comments

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "comments"

// Register adds the rule to the registry.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Flag verbose or agent-memo comments a change touched (advisory) — justify, slim, or remove",
		Doc: "Scans only the comments an edit introduced or modified, so a file full of " +
			"pre-existing prose never fires until you touch it.\n" +
			"BDD markers, linter directives, shebangs and package doc comments are exempt.",
		Events: []hook.EventName{hook.PostToolUse},
		Tools:  []string{"Write", "Edit", "MultiEdit"},
		Config: defaultConfig(),
		// Comment policy is opinionated; blocking on it is too strong a default.
		DefaultSeverity: hook.SeverityAdvisory,
		Check:           checkComments,
	})
}
