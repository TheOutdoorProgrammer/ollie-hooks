package secretscan

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "secret-scan"

// Register adds the rule. UserPromptSubmit is the only event that can stop text
// before it reaches the API — a credential caught at PreToolUse has already
// been sent, and a commit-time gate never sees a pasted one at all.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Catch a credential in your prompt and stop it before it is sent",
		Doc: "`UserPromptSubmit` is the only event that can stop text before it reaches " +
			"the API. By `PreToolUse` the prompt has already gone, and a commit-time " +
			"secret scanner never sees chat at all. This is the last place to catch a " +
			"pasted key.\n" +
			"Findings tell you the kind and the line, never the value. Printing the " +
			"secret into the block reason would drop it straight back into the " +
			"transcript this rule exists to keep it out of.\n" +
			"This one fails closed. If the scan times out you get told, because a " +
			"prompt nobody scanned is exactly the outcome the rule is here to prevent.",
		Events:           []hook.EventName{hook.UserPromptSubmit},
		Config:           defaultConfig(),
		Check:            checkSecrets,
		RequiresBinaries: []hook.Binary{{Bin: "betterleaks", Install: "brew install betterleaks"}},
		// An unscanned prompt is the outcome this rule exists to prevent, so a
		// timeout must be reported rather than passed over.
		FailClosedOnTimeout: true,
	})
}
