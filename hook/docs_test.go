package hook

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type demoNested struct {
	Match  string `toml:"match" doc:"what to match"`
	Reason string `toml:"reason" doc:"why it is protected"`
}

type demoConfig struct {
	Count  int          `toml:"count" doc:"how many"`
	Name   string       `toml:"name" doc:"what to call it"`
	On     bool         `toml:"on" doc:"whether to do it"`
	Items  []string     `toml:"items" doc:"the things"`
	Tables []demoNested `toml:"tables" doc:"nested tables"`
	hidden int          //nolint:unused // unexported fields are not config
}

func demoRule() Rule {
	return Rule{
		ID:          "demo",
		Description: "A demo rule",
		Doc:         "First line.\nSecond line.",
		Events:      []EventName{PostToolUse},
		Check:       func(context.Context, *Event) []Finding { return nil },
		Config: demoConfig{
			Count: 3,
			Name:  "hi",
			On:    true,
			Items: []string{"a", "b"},
		},
	}
}

func TestConfigKeysReflectsDefaults(t *testing.T) {
	keys := demoRule().ConfigKeys()

	want := map[string]any{"count": 3, "name": "hi", "on": true}
	got := map[string]KeyDoc{}
	for _, k := range keys {
		got[k.Key] = k
	}
	for key, def := range want {
		k, ok := got[key]
		if !ok {
			t.Errorf("key %q missing", key)
			continue
		}
		if k.Default != def {
			t.Errorf("%s default = %v, want %v", key, k.Default, def)
		}
		if k.Doc == "" {
			t.Errorf("%s has no doc", key)
		}
	}
	if _, ok := got["hidden"]; ok {
		t.Error("unexported field leaked into the docs")
	}
	if tables := got["tables"].Table; len(tables) != 2 {
		t.Fatalf("nested table keys = %d, want 2", len(tables))
	}
}

func TestUniversalKeysDependOnTheVerb(t *testing.T) {
	check := demoRule().UniversalKeys()
	if !hasKey(check, "severity") {
		t.Error("a Check rule must document severity")
	}

	display := Rule{
		ID: "d", Events: []EventName{MessageDisplay},
		Display: func(context.Context, *Event) *DisplayContent { return nil },
	}
	if hasKey(display.UniversalKeys(), "severity") {
		t.Error("severity documented for a rule that has no findings to route")
	}
	for _, k := range []string{"enabled", "timeout"} {
		if !hasKey(display.UniversalKeys(), k) {
			t.Errorf("every rule must document %q", k)
		}
	}
}

// An undocumented key is unreachable for anyone who did not read the source,
// so it fails at Register — which every rule package's own tests execute.
func TestRegisterRejectsAnUndocumentedKey(t *testing.T) {
	cases := map[string]any{
		"missing doc tag": struct {
			A int `toml:"a"`
		}{},
		"missing toml tag": struct {
			A int `doc:"a"`
		}{},
		"nested missing": struct {
			A []struct {
				B int `toml:"b"`
			} `toml:"a" doc:"a"`
		}{},
		"not a struct": 42,
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			saved := registry
			t.Cleanup(func() { registry = saved })
			registry = nil

			defer func() {
				if recover() == nil {
					t.Error("Register accepted an undocumentable Config")
				}
			}()
			Register(Rule{
				ID: "bad", Events: []EventName{PostToolUse}, Config: cfg,
				Check: func(context.Context, *Event) []Finding { return nil },
			})
		})
	}
}

func TestRegisterAcceptsANilConfig(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil

	Register(Rule{
		ID: "no-config", Events: []EventName{PostToolUse},
		Check: func(context.Context, *Event) []Finding { return nil },
	})
	if len(registry) != 1 {
		t.Fatal("a rule with nothing to configure must still register")
	}
}

// The generated example must be valid TOML, and every key in it must be one
// the rule can actually decode. A documented key the decoder ignores is a lie
// that no amount of prose review would catch.
func TestGeneratedExampleIsDecodableByEveryRule(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil
	Register(demoRule())

	var buf bytes.Buffer
	WriteConfigExample(&buf)

	live := UncommentExample(buf.String())
	writeUserConfig(t, live)

	cfg := demoConfig{}
	if !loadConfig("demo", &cfg) {
		t.Fatalf("the generated example does not parse:\n%s", live)
	}
	if cfg.Count != 3 || cfg.Name != "hi" || !cfg.On {
		t.Errorf("example did not round-trip the defaults: %+v", cfg)
	}
}

func TestGeneratedDocsCoverEveryRule(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil
	Register(demoRule())

	var buf bytes.Buffer
	WriteRuleDocs(&buf)
	out := buf.String()

	for _, want := range []string{
		"## demo", "A demo rule", "First line.", "Second line.",
		"`count`", "`3`", "how many", "`tables.match`", "PostToolUse",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated docs missing %q", want)
		}
	}
}

func hasKey(keys []KeyDoc, name string) bool {
	for _, k := range keys {
		if k.Key == name {
			return true
		}
	}
	return false
}

func TestReplaceSection(t *testing.T) {
	doc := "before\n<!-- x -->\nold\n<!-- /x -->\nafter\n"

	got, err := ReplaceSection(doc, "x", "new")
	if err != nil {
		t.Fatal(err)
	}
	want := "before\n<!-- x -->\nnew\n<!-- /x -->\nafter\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Replacing twice must be stable, or every docs run churns the diff.
	twice, err := ReplaceSection(got, "x", "new")
	if err != nil {
		t.Fatal(err)
	}
	if twice != got {
		t.Errorf("not idempotent:\n%q\n%q", got, twice)
	}
}

func TestReplaceSectionRefusesBadMarkers(t *testing.T) {
	cases := map[string]string{
		"no markers": "just text",
		"no close":   "<!-- x -->\nbody\n",
		"no open":    "body\n<!-- /x -->\n",
		"transposed": "<!-- /x -->\nbody\n<!-- x -->\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ReplaceSection(doc, "x", "new"); err == nil {
				t.Error("expected an error rather than a silently unchanged document")
			}
		})
	}
}
