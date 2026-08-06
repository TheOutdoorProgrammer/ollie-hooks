package bashoutput

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// RuleID is the registry ID and the [rules.<id>] config section name.
const RuleID = "bash-output"

// Register adds the rule. One rule, two events: a non-zero exit lands on
// PostToolUseFailure rather than PostToolUse, but echoing the output is one
// feature and should not be two things for the user to configure.
func Register() {
	hook.Register(hook.Rule{
		ID:          RuleID,
		Description: "Print long Bash output in full instead of making you hit ctrl+o",
		Doc: "Claude Code hides long Bash output behind ctrl+o. This prints the whole " +
			"thing on screen instead.\n" +
			"It only changes what you see. The transcript and Claude's own context keep " +
			"the original either way, so nothing here can lose output.\n" +
			"A failed command gets its own colour, which is usually the one you were " +
			"scrolling for.",
		Events:  []hook.EventName{hook.PostToolUse, hook.PostToolUseFailure},
		Tools:   []string{"Bash"},
		Config:  defaultConfig(),
		Display: displayBashOutput,
	})
}
