package bashoutput

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// config tunes the full-output echo. MinLines keeps short results quiet: only
// output long enough for Claude Code to collapse is worth re-printing.
type config struct {
	MinLines     int    `toml:"min_lines" doc:"only reprint output at least this many lines long"`
	CommandColor string `toml:"command_color" doc:"hex colour for the command line; empty for no colour"`
	FailedColor  string `toml:"failed_color" doc:"hex colour for output of a command that failed"`
	OutputColor  string `toml:"output_color" doc:"hex colour for ordinary output"`
}

func defaultConfig() config {
	return config{
		MinLines:     5,
		CommandColor: hook.DraculaGreen,
		FailedColor:  hook.DraculaRed,
		OutputColor:  hook.DraculaForeground,
	}
}

func loadConfig() config { return hook.Config(RuleID, defaultConfig()) }

// Validate keeps MinLines sane: below 1 it would reprint everything.
func (c *config) Validate() {
	if c.MinLines < 1 {
		c.MinLines = 1
	}
}
