package nocodegen

import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

// protectedPath is one tree the agent may not write to, and why. The reason is
// shown on denial: "you may not edit this" is far less useful than the licence
// or policy that makes it true.
type protectedPath struct {
	Match  string `toml:"match"`
	Reason string `toml:"reason"`
}

// config lists the trees this rule protects. It ships empty: the projects that
// refuse AI-authored code are yours to name, and a shipped default would be us
// guessing at someone else's policy.
type config struct {
	Paths []protectedPath `toml:"paths"`
}

func defaultConfig() config {
	return config{}
}

// loadConfig layers [rules.no-codegen] over the defaults; a malformed section
// falls back to clean defaults.
func loadConfig() config {
	cfg := defaultConfig()
	if !hook.LoadConfig(RuleID, &cfg) {
		return defaultConfig()
	}
	return cfg
}
