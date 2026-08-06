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
		Description: "Run the right linter after an edit and tell Claude what it said",
		Doc: "There are ten or so linters here, picked by file type. That is why you get " +
			"to choose: turning this on should not mean installing a Swift toolchain " +
			"because one file in the repo is Swift.\n" +
			"Leave `linters` empty and it runs whatever you already have. Name one and " +
			"you have made a commitment, so if it is missing you get told rather than " +
			"quietly skipped.\n" +
			"For tools that lint a whole package at a time, like golangci-lint, findings " +
			"are trimmed down to the file you actually edited. Sibling-file noise never " +
			"reaches Claude.\n" +
			"It never blocks the edit. The edit has already happened by the time this " +
			"runs, so the output is context, not a verdict.\n" +
			"\n" +
			"```toml\n" +
			"[rules.lint]\n" +
			"enabled = true\n" +
			"linters = [\"shellcheck\", \"ruff\", \"golangci-lint\"]\n" +
			"```",
		Events:   []hook.EventName{hook.PostToolUse},
		Tools:    []string{"Write", "Edit", "MultiEdit"},
		Config:   defaultConfig(),
		DocTable: docTable,
		Advise:   adviseLint,
	})
}
