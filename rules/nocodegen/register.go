package nocodegen

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "no-codegen"

// Register adds the rule to the registry.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Block writes to trees configured as off-limits to AI-authored code",
		Events:      []hook.EventName{hook.PreToolUse},
		Tools:       []string{"Edit", "Write", "MultiEdit", "NotebookEdit"},
		Check:       checkNoCodegen,
	})
}
