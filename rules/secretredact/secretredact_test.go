package secretredact

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func TestMain(m *testing.M) {
	Register()
	os.Exit(m.Run())
}

// fakePAT builds a token-shaped value that is valid for nothing. Assembled from
// fragments so no whole token literal sits in source, where it would trip every
// secret scanner reading this repo — including our own pre-commit gate.
func fakePAT() string {
	return strings.Join([]string{"ghp", "_A1b2C3d4E5f6", "G7h8I9j0K1l2", "M3n4O5p6Q7r8"}, "")
}

func requireScanner(t *testing.T) {
	t.Helper()
	hooktest.RequireBinary(t, defaultConfig().Binary)
}

func bashResult(t *testing.T, stdout, stderr string) *hook.Event {
	t.Helper()
	resp, err := json.Marshal(map[string]any{
		"stdout": stdout, "stderr": stderr, "interrupted": false, "isImage": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &hook.Event{HookEventName: "PostToolUse", ToolName: "Bash", ToolResponse: resp}
}

func TestRedactsACredentialInStdout(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	m := redactToolOutput(context.Background(), bashResult(t, "export TOKEN="+fakePAT(), ""))
	if m == nil {
		t.Fatal("a credential in stdout must be redacted")
	}
	out, _ := m.UpdatedOutput.(map[string]any)
	got, _ := out["stdout"].(string)
	if strings.Contains(got, fakePAT()) {
		t.Errorf("the secret survived into the output Claude reads: %q", got)
	}
	if !strings.Contains(got, "<redacted:github-pat>") {
		t.Errorf("placeholder should name the kind, got %q", got)
	}
	if !strings.Contains(got, "export TOKEN=") {
		t.Errorf("surrounding text must survive: %q", got)
	}
}

// updatedToolOutput replaces the WHOLE object, so a field the rule never
// touched must still come back or the result silently changes shape.
func TestPreservesUntouchedFields(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	m := redactToolOutput(context.Background(), bashResult(t, fakePAT(), "some stderr"))
	if m == nil {
		t.Fatal("expected a redaction")
	}
	out, ok := m.UpdatedOutput.(map[string]any)
	if !ok {
		t.Fatal("output should stay an object")
	}
	for _, k := range []string{"stdout", "stderr", "interrupted", "isImage"} {
		if _, present := out[k]; !present {
			t.Errorf("field %q was dropped from the tool result", k)
		}
	}
	if got, _ := out["stderr"].(string); got != "some stderr" {
		t.Errorf("clean stderr must be untouched, got %q", got)
	}
}

func TestRedactsStderrToo(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	m := redactToolOutput(context.Background(), bashResult(t, "", "error using "+fakePAT()))
	if m == nil {
		t.Fatal("a credential in stderr must be redacted")
	}
	out, _ := m.UpdatedOutput.(map[string]any)
	if got, _ := out["stderr"].(string); strings.Contains(got, fakePAT()) {
		t.Errorf("secret survived in stderr: %q", got)
	}
}

// The note is how the model learns a value was removed rather than absent.
func TestNoteNamesTheKind(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	m := redactToolOutput(context.Background(), bashResult(t, fakePAT(), ""))
	if m == nil {
		t.Fatal("expected a redaction")
	}
	if !strings.Contains(m.Note, "github-pat") {
		t.Errorf("note should name what was redacted: %q", m.Note)
	}
	if strings.Contains(m.Note, fakePAT()) {
		t.Errorf("the note must not repeat the secret: %q", m.Note)
	}
}

func TestLeavesCleanOutputAlone(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	if m := redactToolOutput(context.Background(), bashResult(t, "total 4\ndrwxr-xr-x", "")); m != nil {
		t.Errorf("clean output needs no rewrite, got %+v", m.UpdatedOutput)
	}
}

func TestIgnoresToolsOutsideTheList(t *testing.T) {
	requireScanner(t)
	hooktest.Config(t, RuleID, "enabled = true")
	ev := bashResult(t, fakePAT(), "")
	ev.ToolName = "Read"
	if m := redactToolOutput(context.Background(), ev); m != nil {
		t.Error("only configured tools should be scanned")
	}
}

func TestFallsOpenOnUnreadableOutput(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	ev := &hook.Event{HookEventName: "PostToolUse", ToolName: "Bash", ToolResponse: []byte("not json")}
	if m := redactToolOutput(context.Background(), ev); m != nil {
		t.Error("an output shape we cannot read must be left alone")
	}
}

func TestDisabledByDefault(t *testing.T) {
	hooktest.NoConfig(t)
	if m := hook.RunRewrites(bashResult(t, fakePAT(), "")); m != nil {
		t.Error("redaction is opt-in; a fresh install must not rewrite output")
	}
}
