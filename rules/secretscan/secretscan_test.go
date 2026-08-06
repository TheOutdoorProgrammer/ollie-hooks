package secretscan

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
	"github.com/TheOutdoorProgrammer/ollie-hooks/internal/betterleaks"
	"github.com/TheOutdoorProgrammer/ollie-hooks/internal/betterleaks/betterleakstest"
)

func TestMain(m *testing.M) {
	Register()
	os.Exit(m.Run())
}

func requireScanner(t *testing.T) {
	t.Helper()
	hooktest.RequireBinary(t, defaultConfig().Binary)
}

func promptEvent(text string) *hook.Event {
	return &hook.Event{HookEventName: "UserPromptSubmit", Prompt: text}
}

func TestBlocksAPromptCarryingACredential(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	got := checkSecrets(context.Background(), promptEvent("please use my token "+betterleakstest.FakePAT()+" to fetch the repo"))
	hooktest.AssertFinding(t, got, "live credential")
}

// The whole point is that the value never reaches the transcript. Repeating it
// in the block reason would put it right back.
func TestNeverEchoesTheSecret(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	got := checkSecrets(context.Background(), promptEvent("token "+betterleakstest.FakePAT()))
	if len(got) == 0 {
		t.Fatal("expected a block")
	}
	for _, f := range got {
		if strings.Contains(f.Message, betterleakstest.FakePAT()) {
			t.Errorf("the secret leaked into the finding: %q", f.Message)
		}
	}
}

func TestAllowsAnOrdinaryPrompt(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	hooktest.AssertClean(t, checkSecrets(context.Background(),
		promptEvent("refactor the login handler to use the new session API")))
}

func TestIgnoresAnEmptyPrompt(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	hooktest.AssertClean(t, checkSecrets(context.Background(), promptEvent("   \n  ")))
}

// Scanning is opt-in: a fresh install must not start blocking prompts.
func TestDisabledByDefault(t *testing.T) {
	hooktest.NoConfig(t)
	hooktest.AssertClean(t, hook.RunChecks(promptEvent("token "+betterleakstest.FakePAT())).Findings)
}

// Fail closed: an enabled scanner that cannot run must say so, not pass.
func TestMissingScannerBlocksLoudly(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true\nbinary = \"definitely-not-a-scanner-xyzzy\"")
	got := checkSecrets(context.Background(), promptEvent("token "+betterleakstest.FakePAT()))
	hooktest.AssertFinding(t, got, "NOT scanned")
}

func TestDescribeNamesTheKindOnce(t *testing.T) {
	got := describe([]betterleaks.Leak{
		{RuleID: "github-pat", StartLine: 1},
		{RuleID: "github-pat", StartLine: 4},
		{RuleID: "aws-key", StartLine: 2},
	})
	if strings.Count(got, "github-pat") != 1 {
		t.Errorf("a repeated kind should be named once: %q", got)
	}
	if !strings.Contains(got, "aws-key") {
		t.Errorf("every distinct kind must be named: %q", got)
	}
}

// A scanner the user repointed is vetted by the framework too, via the rule's
// dynamic Binaries. A static list only ever checked the built-in default, so an
// absent replacement produced no findings at all — indistinguishable from clean.
func TestARepointedScannerIsStillVetted(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true\nbinary = \"definitely-not-a-scanner-xyzzy\"")

	var found []hook.Binary
	for _, r := range hook.Registered() {
		if r.ID == RuleID && r.Binaries != nil {
			found = r.Binaries()
		}
	}
	if len(found) != 1 {
		t.Fatalf("rule should declare the scanner it will actually run, got %v", found)
	}
	if found[0].Bin != "definitely-not-a-scanner-xyzzy" {
		t.Errorf("declared %q, want the configured binary", found[0].Bin)
	}
	if len(hook.MissingBinaries(found)) != 1 {
		t.Error("an absent configured scanner must register as missing")
	}
}
