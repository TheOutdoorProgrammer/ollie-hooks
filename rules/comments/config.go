package comments

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// config tunes the comment gate without a rebuild.
type config struct {
	MaxCommentLines int  `toml:"max_comment_lines"`
	MaxLineLength   int  `toml:"max_line_length"`
	AgentMemo       bool `toml:"agent_memo"`
}

func defaultConfig() config {
	return config{
		MaxCommentLines: 3,
		MaxLineLength:   80,
		AgentMemo:       true,
	}
}

// loadConfig layers [rules.comments] over the defaults. A malformed section
// falls back to clean defaults — a bad config must never half-configure a rule.
func loadConfig() config {
	cfg := defaultConfig()
	if !hook.LoadConfig(RuleID, &cfg) {
		return defaultConfig()
	}
	return cfg
}
