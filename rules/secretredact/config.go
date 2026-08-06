package secretredact

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// config tunes the tool-output redactor. Enabling is the framework's business,
// so there is no Enabled key here.
type config struct {
	Binary string `toml:"binary"`
	// Tools whose output is scanned. Bash is the realistic source — `cat .env`,
	// `env`, a curl response — and scanning every read of every file would cost
	// far more than it catches.
	Tools []string `toml:"tools"`
}

func defaultConfig() config {
	return config{Binary: "betterleaks", Tools: []string{"Bash"}}
}

func loadConfig() config {
	cfg := hook.Config(RuleID, defaultConfig())
	if cfg.Binary == "" {
		cfg.Binary = defaultConfig().Binary
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = defaultConfig().Tools
	}
	return cfg
}
