package rules

import (
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func TestMain(m *testing.M) {
	RegisterAll()
	m.Run()
}

// The generated config must round-trip against every real rule, not just a
// fixture: this is what stops the docs and the decoder drifting apart.
func TestConfigExampleMatchesEveryRule(t *testing.T) {
	hooktest.AssertConfigDocumented(t)
}

// Every rule needs a Doc paragraph. Description is a one-liner for a table;
// the reference page is where someone decides whether to enable the thing.
func TestEveryRuleIsDocumented(t *testing.T) {
	for _, r := range hook.Registered() {
		if r.Description == "" {
			t.Errorf("%s: no Description", r.ID)
		}
		if r.Doc == "" {
			t.Errorf("%s: no Doc — say what it does and when to want it", r.ID)
		}
	}
}
