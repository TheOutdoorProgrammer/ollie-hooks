package prosewrap

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "prose-wrap"

// Register adds the rule to the registry.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Flag markdown your change hard-wrapped mid-sentence",
		Doc: "One sentence per line keeps a diff down to the sentence that changed. Hard " +
			"wrapping means editing one sentence reflows every line after it, and the " +
			"real change gets buried in the noise.\n" +
			"Only lines your change touched are flagged. Code blocks and tables are " +
			"never prose.\n" +
			"Advisory by default. Plenty of people hard-wrap at 80 and are perfectly " +
			"happy, so this is a house style rather than a correctness rule.",
		Events: []hook.EventName{hook.PostToolUse},
		Tools:  []string{"Write", "Edit", "MultiEdit"},
		Config: defaultConfig(),
		// A house style, not a correctness rule — it should read as a suggestion.
		DefaultSeverity: hook.SeverityAdvisory,
		Check:           checkProseWrap,
	})
}
