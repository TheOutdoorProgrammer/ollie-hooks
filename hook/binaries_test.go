package hook

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const fauxBinary = "definitely-not-installed-xyzzy"

// A rule that declares a binary it cannot have. Its Check fails the test if it
// ever runs: a check whose tool is absent must not execute at all, or it would
// report "clean" for work it never did.
func fauxRule(t *testing.T) Rule {
	t.Helper()
	return Rule{
		ID:               "faux-needs-binary",
		Events:           []EventName{PostToolUse},
		EnabledByDefault: true,
		RequiresBinaries: []Binary{{Bin: fauxBinary, Install: "brew install nothing"}},
		Check: func(context.Context, *Event) []Finding {
			t.Error("the check ran despite its binary being missing")
			return nil
		},
	}
}

func TestDeclaredBinaryIsCheckedAndBlocks(t *testing.T) {
	withOnlyRule(t, fauxRule(t))
	writeUserConfig(t, "")

	res := RunChecks(&Event{HookEventName: PostToolUse})
	if len(res.Findings) == 0 {
		t.Fatal("a declared-but-absent binary must produce a blocking finding")
	}
	msg := res.Findings[0].Message
	for _, want := range []string{fauxBinary, "did NOT run", "brew install nothing"} {
		if !strings.Contains(msg, want) {
			t.Errorf("finding should mention %q, got: %s", want, msg)
		}
	}
	if res.Findings[0].Rule != "faux-needs-binary" {
		t.Errorf("finding must name the rule, got %q", res.Findings[0].Rule)
	}
	t.Logf("blocking finding: %s", msg)
}

// Disabled means disabled: the rule is not running, so its tooling is not the
// user's problem and nagging about it would be noise.
func TestDisabledRuleDoesNotDemandItsBinary(t *testing.T) {
	withOnlyRule(t, fauxRule(t))
	writeUserConfig(t, "[rules.faux-needs-binary]\nenabled = false\n")

	if res := RunChecks(&Event{HookEventName: PostToolUse}); len(res.Findings) != 0 {
		t.Errorf("a disabled rule must not demand its binary, got %+v", res.Findings)
	}
}

// The absence must be re-checked every time. Caching a miss would mean
// installing the tool had no effect until the entry expired — exactly when
// someone is waiting for it to start working.
func TestMissingBinaryIsNotCached(t *testing.T) {
	writeUserConfig(t, "")
	bins := []Binary{{Bin: fauxBinary}}

	if len(MissingBinaries(bins)) != 1 {
		t.Fatal("expected the binary to be reported missing")
	}
	if len(MissingBinaries(bins)) != 1 {
		t.Fatal("a second call must still report it missing")
	}

	data, err := os.ReadFile(toolCachePathForTest())
	if err != nil {
		return // no cache written at all is the ideal outcome
	}
	var c binaryCache
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("cache is unreadable: %v", err)
	}
	if _, present := c.Found[fauxBinary]; present {
		t.Error("a missing binary must never be recorded in the cache")
	}
}

func toolCachePathForTest() string { return binaryCachePath() }

// End to end through Decide: the finding has to survive into the JSON envelope
// Claude Code actually reads, not just into RunChecks' return value.
func TestMissingBinaryReachesTheEnvelope(t *testing.T) {
	withOnlyRule(t, fauxRule(t))
	writeUserConfig(t, "")

	out, err := Decide(&Event{HookEventName: PostToolUse, ToolName: "Write"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"decision":"block"`, fauxBinary, "did NOT run"} {
		if !strings.Contains(out, want) {
			t.Errorf("envelope missing %q:\n%s", want, out)
		}
	}
	t.Logf("envelope: %s", out)
}
