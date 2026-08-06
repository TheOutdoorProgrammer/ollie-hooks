package lint

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "lint"

// Register adds the rule to the registry. PostToolUse and Advise, not Check:
// the edit has already landed, so the output is context for the model rather
// than a violation — a block envelope here renders as a hook error.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "After an edit, run the file's linter and report what it found (never blocks the edit)",
		Doc: "Dispatches by file extension to whichever of about ten linters fits, which " +
			"is why the set is configurable: all-or-nothing enabling would demand every " +
			"one of those tools from every user.\n" +
			"Leaving `linters` empty runs whatever happens to be installed; naming one " +
			"is a commitment, so a named linter that is missing gets reported instead " +
			"of quietly skipped.\n" +
			"A package-scoped linter's findings are narrowed to the edited file, so " +
			"sibling-file noise never reaches the model.",
		Events: []hook.EventName{hook.PostToolUse},
		Tools:  []string{"Write", "Edit", "MultiEdit"},
		Config: defaultConfig(),
		Advise: adviseLint,
	})
}
