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

// configuredScanner names the scanner this rule will actually run, which is
// not always the built-in one. Declaring it dynamically lets the framework
// report a missing tool; a static list only ever vets the default, so a
// repointed binary that is absent looks exactly like a clean result.
func configuredScanner() []hook.Binary {
	bin := loadConfig().Binary
	install := "brew install betterleaks"
	if bin != defaultConfig().Binary {
		install = "install " + bin + ", or unset [rules." + RuleID + "].binary"
	}
	return []hook.Binary{{Bin: bin, Install: install}}
}
