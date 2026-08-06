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
		Doc: "It renders while the message is still streaming, so the diagram shows up " +
			"in place rather than arriving after the text it belongs to.\n" +
			"Claude Code's `verbose` setting has to be off. Verbose prints the original " +
			"text, which makes this look like it is doing nothing. That trips people up " +
			"more than anything else here.\n" +
			"Only `graph` and `flowchart`, `sequenceDiagram` and `erDiagram` parse at " +
			"all. Anything else, or anything wider than `width_cap`, keeps its fence and " +
			"is left alone.",
		Events: []hook.EventName{hook.MessageDisplay},
		Config: defaultConfig(),
		// Declare the renderer, so a missing mermaid-ascii is reported by doctor
		// and skips the rule with a trace instead of silently rendering nothing.
		Binaries: func() []hook.Binary {
			return []hook.Binary{{
				Bin:     loadConfig().Binary,
				Install: "go install github.com/AlexanderGrooff/mermaid-ascii@latest",
			}}
		},
		Display: displayMermaidStream,
	})
}
