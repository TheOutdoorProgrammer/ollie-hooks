package hook

import (
	"fmt"
	"os"
	"sync"

	"github.com/BurntSushi/toml"
)

// configFile is the single user config. One file, not one per rule: with every
// rule shipping disabled, users need one place that lists what exists.
const configFile = "config.toml"

// malformedOnce keeps the report below to one line per process, however many
// rules look the file up.
var malformedOnce sync.Once

// decodeConfig reads the config file into root. ok is false only when the file
// exists and does not parse — worth shouting about, because every rule then
// falls back to its default, and every default is disabled.
func decodeConfig(root any) (toml.MetaData, bool) {
	path := configPath(configFile)
	if path == "" {
		return toml.MetaData{}, true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return toml.MetaData{}, true
	}
	md, err := toml.Decode(string(data), root)
	if err != nil {
		malformedOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "ollie-hooks: %s does not parse, so NO rules are "+
				"active — every one fell back to its default, which is disabled: %v\n", path, err)
		})
		return toml.MetaData{}, false
	}
	return md, true
}

// loadConfig decodes the [rules.<id>] section over v's defaults, reporting
// success. Missing file or section is success (v keeps defaults); only a
// malformed section is false, and callers discard v rather than half-apply it.
func loadConfig(id string, v any) bool {
	var root struct {
		Rules map[string]toml.Primitive `toml:"rules"`
	}
	md, ok := decodeConfig(&root)
	if !ok {
		return false
	}
	section, found := root.Rules[id]
	if !found {
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
	if !loadConfig(id, &cfg) || cfg.Enabled == nil {
		return byDefault
	}
	return *cfg.Enabled
}

// Validator lets a config type repair itself after loading, so a rule's
// defaults are enforced once, not by a hand-written dance after every load.
type Validator interface{ Validate() }

// Config loads a rule's [rules.<id>] section over its defaults, then lets the
// result Validate() itself. A malformed section yields the defaults untouched:
// half-applying a broken config is worse than ignoring it.
func Config[T any](id string, defaults T) T {
	cfg := defaults
	if !loadConfig(id, &cfg) {
		return defaults
	}
	if v, ok := any(&cfg).(Validator); ok {
		v.Validate()
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
	if loadConfig(id, &cfg) && cfg.Timeout > 0 {
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
	var root struct {
		Output toml.Primitive `toml:"output"`
	}
	md, ok := decodeConfig(&root)
	if !ok {
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
	var root struct {
		CustomRules map[string]CustomRule `toml:"custom_rules"`
	}
	if _, ok := decodeConfig(&root); !ok {
		return nil
	}
	return root.CustomRules
}
