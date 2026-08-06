package nocodegen

import (
	"context"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

const protectedCfg = `
enabled = true
[[rules.no-codegen.paths]]
match = "/github.com/example/protected/"
reason = "This project does not accept AI-authored contributions."
`

func writeEvent(t *testing.T, tool, field, path string) *hook.Event {
	t.Helper()
	return hooktest.PreToolUse(t, tool, map[string]any{field: path})
}

func TestBlocksWritesUnderAProtectedPath(t *testing.T) {
	cases := []struct {
		name, tool, field, path string
	}{
		{"edit", "Edit", "file_path", "/src/github.com/example/protected/main.go"},
		{"write", "Write", "file_path", "/src/github.com/example/protected/pkg/a.go"},
		{"multiedit", "MultiEdit", "file_path", "/src/github.com/example/protected/b.go"},
		{"notebook", "NotebookEdit", "notebook_path", "/src/github.com/example/protected/n.ipynb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hooktest.Config(t, RuleID, protectedCfg)
			got := checkNoCodegen(context.Background(), writeEvent(t, c.tool, c.field, c.path))
			hooktest.AssertFinding(t, got, "does not accept AI-authored")
		})
	}
}

func TestAllowsWritesElsewhere(t *testing.T) {
	hooktest.Config(t, RuleID, protectedCfg)
	for _, p := range []string{
		"/src/github.com/example/other/main.go",
		"/src/github.com/someoneelse/protected-ish/main.go",
		"/tmp/scratch.go",
	} {
		hooktest.AssertClean(t, checkNoCodegen(context.Background(), writeEvent(t, "Write", "file_path", p)))
	}
}

// Windows tool_input paths use backslashes, so a forward-slash pattern would
// silently never match and the write would proceed unguarded.
func TestMatchesRegardlessOfSeparator(t *testing.T) {
	hooktest.Config(t, RuleID, protectedCfg)
	win := `C:\src\github.com\example\protected\main.go`
	hooktest.AssertFinding(t, checkNoCodegen(context.Background(), writeEvent(t, "Write", "file_path", win)),
		"does not accept AI-authored")
}

// A relative path is resolved against the event's cwd before matching.
func TestResolvesRelativePaths(t *testing.T) {
	hooktest.Config(t, RuleID, protectedCfg)
	ev := writeEvent(t, "Write", "file_path", "main.go")
	ev.CWD = "/src/github.com/example/protected"
	hooktest.AssertFinding(t, checkNoCodegen(context.Background(), ev), "does not accept AI-authored")
}

func TestDisabledByDefault(t *testing.T) {
	hooktest.NoConfig(t)
	hooktest.AssertClean(t, checkNoCodegen(context.Background(),
		writeEvent(t, "Write", "file_path", "/src/github.com/example/protected/main.go")))
}

// Enabled with no paths protects nothing — there is no implied default list.
func TestEnabledWithNoPathsBlocksNothing(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	hooktest.AssertClean(t, checkNoCodegen(context.Background(),
		writeEvent(t, "Write", "file_path", "/src/github.com/example/protected/main.go")))
}

func TestFallsOpenOnUnreadableInput(t *testing.T) {
	hooktest.Config(t, RuleID, protectedCfg)
	ev := &hook.Event{HookEventName: "PreToolUse", ToolName: "Write", ToolInput: []byte("not json")}
	hooktest.AssertClean(t, checkNoCodegen(context.Background(), ev))
}

// The reason from config is what the model reads; without one it still has to
// say something actionable rather than a bare refusal.
func TestFallsBackToAGenericReason(t *testing.T) {
	hooktest.Config(t, RuleID, `
enabled = true
[[rules.no-codegen.paths]]
match = "/github.com/example/protected/"
`)
	got := checkNoCodegen(context.Background(), writeEvent(t, "Write", "file_path", "/src/github.com/example/protected/a.go"))
	hooktest.AssertFinding(t, got, "off-limits to AI-authored code")
}
