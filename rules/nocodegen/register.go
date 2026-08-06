package nocodegen

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "no-codegen"

// Register adds the rule to the registry.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Stop Claude writing to directories you have marked off-limits",
		Doc: "Out of the box this protects nothing. Which projects refuse AI-authored " +
			"code is your call, and any list shipped here would be a guess about " +
			"someone else's licence.\n" +
			"Matching is a plain substring against the full path. Blunt on purpose, but " +
			"worth knowing: `match = \"opentofu\"` also blocks a directory called " +
			"`not-opentofu-notes`.\n" +
			"Write a real `reason`. It is what Claude gets told, and \"you may not edit " +
			"this\" helps nobody.",
		Events: []hook.EventName{hook.PreToolUse},
		Tools:  []string{"Edit", "Write", "MultiEdit", "NotebookEdit"},
		Config: defaultConfig(),
		Check:  checkNoCodegen,
	})
}
