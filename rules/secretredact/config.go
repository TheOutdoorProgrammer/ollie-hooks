package secretredact

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// config tunes the tool-output redactor. Enabling is the framework's business,
// so there is no Enabled key here.
type config struct {
	Binary string `toml:"binary" doc:"which scanner to run"`
	// Bash is the realistic source (cat .env, env, a curl response); scanning
	// every read of every file would cost far more than it catches.
	Tools []string `toml:"tools" doc:"tools whose output is scanned"`
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
