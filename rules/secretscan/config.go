package secretscan

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// config tunes the prompt scanner.
type config struct {
	Binary string `toml:"binary" doc:"which scanner to run"`
	// Full redaction by default: this rule reports that a credential was found,
	// never the credential, or blocking it would be self-defeating.
	Redact int `toml:"redact" doc:"how much of a matched secret to hide, as a percent"`
}

func defaultConfig() config {
	return config{Binary: "betterleaks", Redact: 100}
}

// loadConfig layers [rules.secret-scan] over the defaults; a malformed section
// falls back to clean defaults.
func loadConfig() config {
	cfg := defaultConfig()
	if !hook.LoadConfig(RuleID, &cfg) {
		return defaultConfig()
	}
	if cfg.Binary == "" {
		cfg.Binary = defaultConfig().Binary
	}
	if cfg.Redact < 0 || cfg.Redact > 100 {
		cfg.Redact = defaultConfig().Redact
	}
	return cfg
}
