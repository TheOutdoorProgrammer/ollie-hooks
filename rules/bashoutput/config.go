package bashoutput

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// config tunes the full-output echo. MinLines keeps short results quiet: only
// output long enough for Claude Code to collapse is worth re-printing.
type config struct {
	MinLines     int    `toml:"min_lines" doc:"only re-print output at least this many lines long"`
	CommandColor string `toml:"command_color" doc:"hex triple for the echoed command; empty disables colour"`
	FailedColor  string `toml:"failed_color" doc:"hex triple for output of a command that failed"`
	OutputColor  string `toml:"output_color" doc:"hex triple for ordinary output"`
}

func defaultConfig() config {
	return config{
		MinLines:     5,
		CommandColor: "50fa7b",
		FailedColor:  "ff5555",
		OutputColor:  "f8f8f2",
	}
}

// loadConfig layers [rules.bash-output] over the defaults; a malformed section
// falls back to clean defaults.
func loadConfig() config {
	cfg := defaultConfig()
	if !hook.LoadConfig(RuleID, &cfg) {
		return defaultConfig()
	}
	if cfg.MinLines < 1 {
		cfg.MinLines = 1
	}
	return cfg
}
