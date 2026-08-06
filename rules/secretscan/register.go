package secretscan

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "secret-scan"

// Register adds the rule. UserPromptSubmit is the only event that can stop text
// before it reaches the API — a credential caught at PreToolUse has already
// been sent, and a commit-time gate never sees a pasted one at all.
func Register() {
	hook.Register(hook.Rule{
		ID:               RuleID,
		Description:      "Scan the prompt for credentials and block before it is sent",
		Events:           []hook.EventName{hook.UserPromptSubmit},
		Check:            checkSecrets,
		RequiresBinaries: []hook.Binary{{Bin: "betterleaks", Install: "brew install betterleaks"}},
		// An unscanned prompt is the outcome this rule exists to prevent, so a
		// timeout must be reported rather than passed over.
		FailClosedOnTimeout: true,
	})
}
