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

// loadConfig layers [rules.mermaid-stream] over the defaults; a malformed
// section falls back to clean defaults.
func loadConfig() config {
	cfg := defaultConfig()
	if !hook.LoadConfig(RuleID, &cfg) {
		return defaultConfig()
	}
	if cfg.WidthCap <= 0 {
		cfg.WidthCap = defaultConfig().WidthCap
	}
	if cfg.Binary == "" {
		cfg.Binary = defaultConfig().Binary
	}
	return cfg
}
