package secretredact

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "secret-redact"

// Register adds the rule. PostToolUse is the documented place to intercept
// inbound results: once a credential reaches Claude's context it is in the
// transcript permanently, and nothing downstream can take it back.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Strip credentials out of tool output before Claude ever sees them",
		Doc: "The other half of `secret-scan`. That one watches what you type; this one " +
			"watches what your tools hand back. `cat .env`, `env`, a curl response with " +
			"a bearer token in it.\n" +
			"Once a secret is in Claude's context it is in the transcript for good, and " +
			"nothing downstream gets it back.\n" +
			"It matches the literal value rather than the column offsets the scanner " +
			"reports. Column arithmetic falls apart the moment output has multibyte " +
			"characters in it, and a half-redacted secret is worse than none because it " +
			"looks like it worked.",
		Events:           []hook.EventName{hook.PostToolUse},
		Config:           defaultConfig(),
		Rewrite:          redactToolOutput,
		RequiresBinaries: []hook.Binary{{Bin: "betterleaks", Install: "brew install betterleaks"}},
	})
}
