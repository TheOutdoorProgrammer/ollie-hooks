package hook

import (
	"os"

	"github.com/BurntSushi/toml"
)

// configFile is the single user config. One file, not one per rule: with every
// rule shipping disabled, users need one place that lists what exists.
const configFile = "config.toml"

// LoadConfig decodes the [rules.<id>] section over whatever defaults v already
// holds, and reports whether that succeeded. A missing file or absent section
// is success — v keeps its defaults. Only a malformed section is false, which
// callers answer by discarding v: a bad config must never half-configure a rule.
func LoadConfig(id string, v any) bool {
	path := configPath(configFile)
	if path == "" {
		return true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var root struct {
		Rules map[string]toml.Primitive `toml:"rules"`
	}
	md, err := toml.Decode(string(data), &root)
	if err != nil {
		return false
	}
	section, ok := root.Rules[id]
	if !ok {
		return true
	}
	return md.PrimitiveDecode(section, v) == nil
}

// RuleEnabled reports whether [rules.<id>].enabled turns a rule on, falling
// back to the rule's own default when the key is absent.
func RuleEnabled(id string, byDefault bool) bool {
	cfg := struct {
		Enabled *bool `toml:"enabled"`
	}{}
	if !LoadConfig(id, &cfg) || cfg.Enabled == nil {
		return byDefault
	}
	return *cfg.Enabled
}

// Config loads a rule's [rules.<id>] section over its defaults. A malformed
// section yields the defaults untouched: half-applying a broken config is worse
// than ignoring it, because the rule then behaves in a way nobody chose.
func Config[T any](id string, defaults T) T {
	cfg := defaults
	if !LoadConfig(id, &cfg) {
		return defaults
	}
	return cfg
}

// RuleTimeout is a rule's in-process budget: [rules.<id>].timeout wins, then
// the rule's own, then the default. A linter that is fast on one repo is slow
// on a monorepo, and only the person running it knows which.
func RuleTimeout(id string, declared int) int {
	cfg := struct {
		Timeout int `toml:"timeout"`
	}{}
	if LoadConfig(id, &cfg) && cfg.Timeout > 0 {
		return cfg.Timeout
	}
	if declared > 0 {
		return declared
	}
	return defaultRuleTimeout
}

// wrapWidth is the finding-row budget from [output], or the built-in default.
func wrapWidth() int {
	var out struct {
		WrapWidth int `toml:"wrap_width"`
	}
	path := configPath(configFile)
	if path == "" {
		return defaultWrapWidth
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultWrapWidth
	}
	var root struct {
		Output toml.Primitive `toml:"output"`
	}
	md, err := toml.Decode(string(data), &root)
	if err != nil {
		return defaultWrapWidth
	}
	if err := md.PrimitiveDecode(root.Output, &out); err != nil || out.WrapWidth <= 0 {
		return defaultWrapWidth
	}
	return out.WrapWidth
}

// CustomRule is one [custom_rules.<id>] entry: a rule in another process,
// speaking the same JSON this hook already receives. Verb is a capability
// declaration — it picks which reply field is read, so a plugin enabled to
// advise cannot start denying calls after an update you did not read.
type CustomRule struct {
	// StartupCmd runs the plugin per call: request on stdin, reply on stdout.
	StartupCmd string `toml:"startup_cmd"`
	// ServerURL posts to an already-running plugin, for one needing warm state.
	ServerURL string      `toml:"server_url"`
	Verb      Verb        `toml:"verb"`
	Events    []EventName `toml:"events"`
	Tools     []string    `toml:"tools"`
	Timeout   int         `toml:"timeout"`
	Enabled   bool        `toml:"enabled"`
}

// LoadCustomRules returns the configured out-of-process rules by id.
func LoadCustomRules() map[string]CustomRule {
	path := configPath(configFile)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var root struct {
		CustomRules map[string]CustomRule `toml:"custom_rules"`
	}
	if _, err := toml.Decode(string(data), &root); err != nil {
		return nil
	}
	return root.CustomRules
}
