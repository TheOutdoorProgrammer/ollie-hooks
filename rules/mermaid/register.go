package mermaid

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "mermaid-stream"

// Register adds the rule to the registry. MessageDisplay fires once per
// streamed delta, so this is the hottest rule in the set.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Render ```mermaid fences as ASCII inline while text streams (needs verbose:false)",
		Events:      []hook.EventName{hook.MessageDisplay},
		Display:     displayMermaidStream,
	})
}
