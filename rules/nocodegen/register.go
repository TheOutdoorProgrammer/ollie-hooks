package nocodegen

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "no-codegen"

// Register adds the rule to the registry.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Block writes to trees configured as off-limits to AI-authored code",
		Doc: "Ships protecting nothing: the projects that refuse AI-authored code are " +
			"yours to name, and a shipped default would be guesswork about someone " +
			"else's policy.\n" +
			"Matching is a plain substring against the normalised absolute path, so " +
			"`match = \"opentofu\"` also blocks a sibling directory merely named " +
			"`not-opentofu-notes`.",
		Events: []hook.EventName{hook.PreToolUse},
		Tools:  []string{"Edit", "Write", "MultiEdit", "NotebookEdit"},
		Config: defaultConfig(),
		Check:  checkNoCodegen,
	})
}
