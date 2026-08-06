package secretredact

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "secret-redact"

// Register adds the rule. PostToolUse is the documented place to intercept
// inbound results: once a credential reaches Claude's context it is in the
// transcript permanently, and nothing downstream can take it back.
func Register() {
	hook.Register(hook.Rule{
		ID:               RuleID,
		Description:      "Replace credentials in tool output with a placeholder before Claude reads it",
		Events:           []hook.EventName{hook.PostToolUse},
		Rewrite:          redactToolOutput,
		RequiresBinaries: []hook.Binary{{Bin: "betterleaks", Install: "brew install betterleaks"}},
	})
}
