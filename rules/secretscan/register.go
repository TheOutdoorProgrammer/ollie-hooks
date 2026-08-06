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
		Description: "Scan the prompt for credentials and block before it is sent",
		Doc: "UserPromptSubmit is the only event that can stop text before it reaches " +
			"the API, so this is the last point a pasted credential can still be " +
			"caught — PreToolUse is already too late, and a commit-time gate never " +
			"sees chat at all.\n" +
			"Findings name the kind and line, never the value: repeating a secret into " +
			"the block reason would put it straight back into the transcript this rule " +
			"exists to keep it out of.",
		Events:           []hook.EventName{hook.UserPromptSubmit},
		Config:           defaultConfig(),
		Check:            checkSecrets,
		RequiresBinaries: []hook.Binary{{Bin: "betterleaks", Install: "brew install betterleaks"}},
		// An unscanned prompt is the outcome this rule exists to prevent, so a
		// timeout must be reported rather than passed over.
		FailClosedOnTimeout: true,
	})
}
