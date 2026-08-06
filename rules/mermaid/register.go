package mermaid

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "mermaid-stream"

// Register adds the rule to the registry. MessageDisplay fires once per
// streamed delta, so this is the hottest rule in the set.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Render mermaid fences as terminal ASCII, inline while text streams",
		Doc: "Renders as the message streams, so a diagram appears inline rather than " +
			"after the fact.\n" +
			"Needs Claude Code's `verbose` setting OFF — verbose shows the original " +
			"text, which makes this rule look broken.\n" +
			"Only `graph`/`flowchart`, `sequenceDiagram` and `erDiagram` parse; anything " +
			"else, or anything wider than `width_cap`, keeps its fence unchanged.",
		Events:  []hook.EventName{hook.MessageDisplay},
		Config:  defaultConfig(),
		Display: displayMermaidStream,
	})
}
