package mermaid

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// config tunes the mermaid rules. WidthCap exists because mermaid-ascii has no
// max-width logic: past it a render is wrapped noise, so the fence is kept.
type config struct {
	WidthCap int    `toml:"width_cap" doc:"give up and keep the fence past this many columns"`
	Binary   string `toml:"binary" doc:"which renderer to run"`
}

func defaultConfig() config {
	return config{WidthCap: 120, Binary: "mermaid-ascii"}
}

func loadConfig() config { return hook.Config(RuleID, defaultConfig()) }

// Validate refills any emptied field so a partial section still renders.
func (c *config) Validate() {
	d := defaultConfig()
	if c.WidthCap <= 0 {
		c.WidthCap = d.WidthCap
	}
	if c.Binary == "" {
		c.Binary = d.Binary
	}
}
