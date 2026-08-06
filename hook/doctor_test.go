package hook

import (
	"context"
	"slices"
	"testing"
)

// demoConfigRule registers a rule with a configurable key, so a strict parse
// has something real to accept and a typo of it to reject.
func demoConfigRule() Rule {
	return Rule{
		ID: "demo", Events: []EventName{PostToolUse},
		Config: struct {
			Limit int `toml:"limit" doc:"how many"`
		}{Limit: 3},
		Check: func(context.Context, *Event) []Finding { return nil },
	}
}

func TestStrayKeyIsReported(t *testing.T) {
	withRules(t, demoConfigRule())
	writeUserConfig(t, "[rules.demo]\nlimit = 5\nmax_commet_lines = 9\n")

	_, _, parses, _, stray, unknown := diagnoseConfig()
	if !parses {
		t.Fatal("config should parse")
	}
	if !slices.Contains(stray, "rules.demo.max_commet_lines") {
		t.Errorf("the misspelled key should be stray, got %v", stray)
	}
	if len(unknown) != 0 {
		t.Errorf("no unknown rule expected, got %v", unknown)
	}
}

func TestUnknownRuleSectionIsReported(t *testing.T) {
	withRules(t, demoConfigRule())
	writeUserConfig(t, "[rules.nope]\nfoo = 1\n")

	_, _, _, _, stray, unknown := diagnoseConfig()
	if !slices.Contains(unknown, "nope") {
		t.Errorf("[rules.nope] should be unknown, got %v", unknown)
	}
	// Its inner keys must not also read as stray: it is one problem, not two.
	if slices.Contains(stray, "rules.nope.foo") {
		t.Errorf("an unknown rule's keys must not be stray too, got %v", stray)
	}
}

func TestCleanConfigHasNoStrayOrUnknown(t *testing.T) {
	withRules(t, demoConfigRule())
	writeUserConfig(t, "[rules.demo]\nenabled = true\nlimit = 5\n")

	_, _, parses, _, stray, unknown := diagnoseConfig()
	if !parses || len(stray) != 0 || len(unknown) != 0 {
		t.Errorf("clean config: parses=%v stray=%v unknown=%v", parses, stray, unknown)
	}
}

func TestMalformedConfigReportedByDiagnose(t *testing.T) {
	withRules(t, demoConfigRule())
	writeUserConfig(t, "this is not toml {{{")
	writeSettings(t, `{"hooks":{"PostToolUse":[{"hooks":[{"command":"/x/ollie-hooks"}]}]}}`)

	d := Diagnose()
	if d.ConfigParses {
		t.Error("a malformed config must report ConfigParses false")
	}
	if d.OK() {
		t.Error("a malformed config is a problem; OK must be false")
	}
}

func TestNoConfigIsClean(t *testing.T) {
	withRules(t, demoConfigRule())
	writeUserConfig(t, "")
	writeSettings(t, `{"hooks":{"PostToolUse":[{"hooks":[{"command":"/x/ollie-hooks"}]}]}}`)

	d := Diagnose()
	if d.ConfigExists {
		t.Error("no file was written; ConfigExists must be false")
	}
	if !d.OK() {
		t.Errorf("no config plus full wiring should be clean, got wiring %v stray %v", d.Wiring.Missing, d.StrayKeys)
	}
}

// The effective table must reflect config, not just the rule's default: a rule
// that ships disabled but is enabled in config should read enabled.
func TestDiagnoseEffectiveRuleState(t *testing.T) {
	withRules(t, demoConfigRule())
	writeUserConfig(t, "[rules.demo]\nenabled = true\nseverity = \"advisory\"\n")
	writeSettings(t, `{"hooks":{"PostToolUse":[{"hooks":[{"command":"/x/ollie-hooks"}]}]}}`)

	d := Diagnose()
	if len(d.Rules) != 1 {
		t.Fatalf("want 1 rule status, got %d", len(d.Rules))
	}
	st := d.Rules[0]
	if !st.Enabled {
		t.Error("demo is enabled in config; status must be enabled")
	}
	if st.Severity != SeverityAdvisory {
		t.Errorf("severity = %q, want advisory", st.Severity)
	}
}
