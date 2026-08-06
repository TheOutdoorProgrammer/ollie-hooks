package hook

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/BurntSushi/toml"
)

type demoCfg struct {
	Value string `toml:"value"`
}

// resetProjectCache clears the once-per-process project decode, so each test
// sees its own working directory and trust state.
func resetProjectCache() {
	projectOnce = sync.Once{}
	projectData = projectRoot{}
	projectMD = toml.MetaData{}
	projectOK = false
	untrustOnce = sync.Once{}
	malformedPrj = sync.Once{}
	customRulesOnce = sync.Once{}
}

// setupProject writes a user config and a project .ollie-hooks.toml, then makes
// the project directory the working dir. It returns the project directory.
func setupProject(t *testing.T, userBody, projectBody string) string {
	t.Helper()
	writeUserConfig(t, userBody)
	proj := t.TempDir()
	if projectBody != "" {
		p := filepath.Join(proj, projectConfigFile)
		if err := os.WriteFile(p, []byte(projectBody), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(proj)
	resetProjectCache()
	t.Cleanup(resetProjectCache)
	return proj
}

func TestUntrustedProjectConfigIsIgnored(t *testing.T) {
	setupProject(t, "[rules.demo]\nvalue = \"user\"\n", "[rules.demo]\nvalue = \"project\"\n")
	var cfg demoCfg
	loadConfig("demo", &cfg)
	if cfg.Value != "user" {
		t.Errorf("untrusted project config must not apply, got %q", cfg.Value)
	}
}

func TestTrustedProjectConfigApplies(t *testing.T) {
	proj := setupProject(t, "[rules.demo]\nvalue = \"user\"\n", "[rules.demo]\nvalue = \"project\"\n")
	Trust(proj)
	resetProjectCache()
	var cfg demoCfg
	loadConfig("demo", &cfg)
	if cfg.Value != "project" {
		t.Errorf("trusted project config should win, got %q", cfg.Value)
	}
}

// Even a trusted repo cannot weaken credential scanning: secret-scan takes
// config from the user file only.
func TestSecretRulesAreFloored(t *testing.T) {
	proj := setupProject(t,
		"[rules.secret-scan]\nbinary = \"betterleaks\"\n",
		"[rules.secret-scan]\nbinary = \"attacker-tool\"\n")
	Trust(proj)
	resetProjectCache()
	var cfg struct {
		Binary string `toml:"binary"`
	}
	loadConfig("secret-scan", &cfg)
	if cfg.Binary != "betterleaks" {
		t.Errorf("secret-scan must ignore project config, got binary %q", cfg.Binary)
	}
}

// Any rule that runs an external program is floored too, so a trusted repo can
// never repoint a scanner or linter at an arbitrary binary.
func TestBinaryRulesAreFloored(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = []Rule{{
		ID: "needs-bin", Events: []EventName{PostToolUse},
		RequiresBinaries: []Binary{{Bin: "some-tool"}},
		Check:            func(context.Context, *Event) []Finding { return nil },
	}}
	proj := setupProject(t, "[rules.needs-bin]\nvalue = \"user\"\n", "[rules.needs-bin]\nvalue = \"project\"\n")
	Trust(proj)
	resetProjectCache()
	if !isFloorRule("needs-bin") {
		t.Fatal("a rule declaring binaries must be floored")
	}
	var cfg demoCfg
	loadConfig("needs-bin", &cfg)
	if cfg.Value != "user" {
		t.Errorf("a binary-running rule must ignore project config, got %q", cfg.Value)
	}
}

// A startup_cmd is arbitrary local code, so project config never defines a
// custom rule, even when the repo is trusted.
func TestCustomRulesNeverComeFromProject(t *testing.T) {
	proj := setupProject(t, "",
		"[custom_rules.evil]\nenabled = true\nstartup_cmd = \"touch /tmp/pwned\"\nverb = \"Check\"\n")
	Trust(proj)
	resetProjectCache()
	if _, ok := LoadCustomRules()["evil"]; ok {
		t.Error("project config must never contribute a custom rule")
	}
}
