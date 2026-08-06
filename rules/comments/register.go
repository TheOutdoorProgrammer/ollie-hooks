package comments

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "comments"

// Register adds the rule to the registry.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Flag bloated or self-narrating comments your change touched",
		Doc: "Only comments your edit actually touched get flagged. A file full of old " +
			"prose stays quiet until you go near it.\n" +
			"The three triggers are length, line width, and comments that narrate the " +
			"change instead of explaining the code. \"Changed X to Y\" is git's job.\n" +
			"BDD markers, linter directives, shebangs and package doc comments are all " +
			"exempt.\n" +
			"Advisory by default, because comment style is an opinion and blocking on " +
			"an opinion is obnoxious. Set `severity` if you want it enforced.",
		Events: []hook.EventName{hook.PostToolUse},
		Tools:  []string{"Write", "Edit", "MultiEdit"},
		Config: defaultConfig(),
		// Comment policy is opinionated; blocking on it is too strong a default.
		DefaultSeverity: hook.SeverityAdvisory,
		Check:           checkComments,
	})
}
