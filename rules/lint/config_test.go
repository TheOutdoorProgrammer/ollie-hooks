package lint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func TestSelectedDefaultsToWhateverIsInstalled(t *testing.T) {
	c := config{}
	run, named := c.selected("shellcheck")
	if !run || named {
		t.Errorf("an empty list should run everything unnamed, got run=%v named=%v", run, named)
	}
}

func TestSelectedHonoursAnExplicitList(t *testing.T) {
	c := config{Linters: []string{"ruff", "shellcheck"}}
	if run, named := c.selected("ruff"); !run || !named {
		t.Errorf("a listed linter must run and count as named, got %v/%v", run, named)
	}
	if run, _ := c.selected("golangci-lint"); run {
		t.Error("an unlisted linter must not run")
	}
}

// Naming a linter is a request for it to run, so its absence is a gap to
// report — not the silent skip an unnamed one gets.
func TestNamedButMissingLinterIsReported(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true\nlinters = [\"definitely-not-real\"]")
	dir := t.TempDir()
	target := filepath.Join(dir, "x.fixture")
	if err := os.WriteFile(target, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lintersByExt[".fixture"] = linterSpec{name: "fake", cmd: "definitely-not-real"}
	defer delete(lintersByExt, ".fixture")

	in, _ := json.Marshal(map[string]string{"file_path": target})
	got := adviseLint(context.Background(), &hook.Event{
		HookEventName: hook.PostToolUse, ToolName: "Write", ToolInput: in,
	})
	if got == nil || !strings.Contains(got.Text, "not installed") {
		t.Errorf("a named-but-absent linter must be reported, got %v", got)
	}
}

func TestUnnamedMissingLinterStaysQuiet(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	dir := t.TempDir()
	target := filepath.Join(dir, "y.fixture")
	if err := os.WriteFile(target, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lintersByExt[".fixture"] = linterSpec{name: "fake", cmd: "definitely-not-real"}
	defer delete(lintersByExt, ".fixture")

	in, _ := json.Marshal(map[string]string{"file_path": target})
	if got := adviseLint(context.Background(), &hook.Event{
		HookEventName: hook.PostToolUse, ToolName: "Write", ToolInput: in,
	}); got != nil {
		t.Errorf("never nag about a tool the user did not ask for, got %q", got.Text)
	}
}

func TestInstallHintNamesTheRightPackageManager(t *testing.T) {
	if h := installHint("markdownlint-cli2"); !strings.Contains(h, "npm") {
		t.Errorf("markdownlint-cli2 is npm-only, got %q", h)
	}
	if h := installHint("tofu"); !strings.Contains(h, "opentofu") {
		t.Errorf("the tofu binary comes from the opentofu formula, got %q", h)
	}
	if h := installHint("something-else"); h == "" {
		t.Error("an unknown linter still needs some hint")
	}
}
