package comments

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// config tunes the comment gate without a rebuild.
type config struct {
	MaxCommentLines int  `toml:"max_comment_lines" doc:"flag comment blocks taller than this"`
	MaxLineLength   int  `toml:"max_line_length" doc:"flag comment lines longer than this (runes, not bytes)"`
	AgentMemo       bool `toml:"agent_memo" doc:"flag comments narrating the change itself (changed X to Y, Note:)"`
}

func defaultConfig() config {
	return config{
		MaxCommentLines: 3,
		MaxLineLength:   80,
		AgentMemo:       true,
	}
}

func loadConfig() config { return hook.Config(RuleID, defaultConfig()) }
