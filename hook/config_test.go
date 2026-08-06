package hook

import (
	"context"
	"strings"
	"testing"
)

// A malformed file must not read as "no config". Every rule would fall back to
// its default, every default is disabled, and the gate would be silently off.
func TestMalformedConfigIsNotMistakenForAbsent(t *testing.T) {
	writeUserConfig(t, "[rules.demo]\nenabled = \"unterminated\n")

	var cfg struct {
		Enabled bool `toml:"enabled"`
	}
	if loadConfig("demo", &cfg) {
		t.Error("loadConfig reported success for a file that does not parse")
	}
}

func TestMalformedConfigLeavesDefaultsIntact(t *testing.T) {
	writeUserConfig(t, "this is not toml {{{")

	defaults := struct {
		Limit int `toml:"limit"`
	}{Limit: 42}

	if got := Config("demo", defaults); got.Limit != 42 {
		t.Errorf("Limit = %d, want the default 42 kept when the file is broken", got.Limit)
	}
}

func TestValidConfigStillLoads(t *testing.T) {
	writeUserConfig(t, "[rules.demo]\nlimit = 7\n")

	got := Config("demo", struct {
		Limit int `toml:"limit"`
	}{Limit: 42})
	if got.Limit != 7 {
		t.Errorf("Limit = %d, want 7", got.Limit)
	}
}

func TestAbsentSectionKeepsDefaults(t *testing.T) {
	writeUserConfig(t, "[rules.other]\nlimit = 7\n")

	got := Config("demo", struct {
		Limit int `toml:"limit"`
	}{Limit: 42})
	if got.Limit != 42 {
		t.Errorf("Limit = %d, want the default 42", got.Limit)
	}
}

// Naming a custom rule after a built-in is the obvious way to try to override
// one. Register panics on a duplicate id, and main recovers by exiting 0, so
// letting that panic through would disable every rule at once.
func TestCustomRuleCannotCollideWithABuiltIn(t *testing.T) {
	withOnlyRule(t, Rule{
		ID: "taken", Events: []EventName{PostToolUse},
		Check: func(context.Context, *Event) []Finding { return nil },
	})
	writeUserConfig(t, `
[custom_rules.taken]
enabled     = true
startup_cmd = "true"
verb        = "Check"
events      = ["PostToolUse"]
`)

	errs := RegisterCustomRules()

	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "built-in") {
		t.Errorf("error should explain the collision, got: %v", errs[0])
	}
	if len(registry) != 1 {
		t.Errorf("registry has %d rules, want the 1 built-in left intact", len(registry))
	}
}

func TestCustomRuleWithAFreeIDStillRegisters(t *testing.T) {
	withOnlyRule(t, Rule{
		ID: "taken", Events: []EventName{PostToolUse},
		Check: func(context.Context, *Event) []Finding { return nil },
	})
	writeUserConfig(t, `
[custom_rules.free]
enabled     = true
startup_cmd = "true"
verb        = "Check"
events      = ["PostToolUse"]
`)

	if errs := RegisterCustomRules(); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(registry) != 2 {
		t.Errorf("registry has %d rules, want 2", len(registry))
	}
}
