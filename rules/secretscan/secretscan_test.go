package secretscan

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

// fakePAT builds a token-shaped value that is valid for nothing. Assembled from
// fragments so no whole token literal sits in source, where it would trip every
// secret scanner reading this repo — including our own pre-commit gate.
func fakePAT() string {
	return strings.Join([]string{"ghp", "_A1b2C3d4E5f6", "G7h8I9j0K1l2", "M3n4O5p6Q7r8"}, "")
}

func TestMain(m *testing.M) {
	Register()
	os.Exit(m.Run())
}

func requireScanner(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(defaultConfig().Binary); err != nil {
		t.Skip("betterleaks not installed")
	}
}

func promptEvent(text string) *hook.Event {
	return &hook.Event{HookEventName: "UserPromptSubmit", Prompt: text}
}

func TestBlocksAPromptCarryingACredential(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	got := checkSecrets(context.Background(), promptEvent("please use my token "+fakePAT()+" to fetch the repo"))
	hooktest.AssertFinding(t, got, "live credential")
}

// The whole point is that the value never reaches the transcript. Repeating it
// in the block reason would put it right back.
func TestNeverEchoesTheSecret(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	got := checkSecrets(context.Background(), promptEvent("token "+fakePAT()))
	if len(got) == 0 {
		t.Fatal("expected a block")
	}
	for _, f := range got {
		if strings.Contains(f.Message, fakePAT()) {
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
	hooktest.AssertClean(t, hook.Run(promptEvent("token "+fakePAT())))
}

// Fail closed: an enabled scanner that cannot run must say so, not pass.
func TestMissingScannerBlocksLoudly(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true\nbinary = \"definitely-not-a-scanner-xyzzy\"")
	got := checkSecrets(context.Background(), promptEvent("token "+fakePAT()))
	hooktest.AssertFinding(t, got, "not installed")
}

func TestParseLeaksIgnoresJunk(t *testing.T) {
	for _, in := range []string{"", "   ", "not json", "{}", "INF scanned 12 bytes"} {
		if got := parseLeaks(in); got != nil {
			t.Errorf("parseLeaks(%q) = %v, want nil", in, got)
		}
	}
}

func TestDescribeNamesTheKindOnce(t *testing.T) {
	got := describe([]leak{
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
