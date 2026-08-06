package bashoutput

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func bashResultEvent(stdout, stderr string) *hook.Event {
	resp, _ := json.Marshal(map[string]any{"stdout": stdout, "stderr": stderr})
	in, _ := json.Marshal(map[string]any{"command": "seq 1 20"})
	return &hook.Event{
		HookEventName: "PostToolUse",
		ToolName:      "Bash",
		ToolInput:     in,
		ToolResponse:  resp,
	}
}

func longOutput(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line"
	}
	return strings.Join(lines, "\n")
}

func TestBashOutputEchoesLongResult(t *testing.T) {
	dc := displayBashOutput(context.Background(), bashResultEvent(longOutput(20), ""))
	if dc == nil {
		t.Fatal("long output should be echoed")
	}
	if got := strings.Count(dc.Text, "line"); got != 20 {
		t.Errorf("every line must survive — no cap: got %d of 20", got)
	}
	// Leading newline so the "says:" prefix doesn't share a line with output.
	if !strings.HasPrefix(dc.Text, "\n") {
		t.Errorf("echo must start on its own line:\n%s", dc.Text)
	}
	if !strings.Contains(dc.Text, "$ seq 1 20") {
		t.Errorf("echo must name the command:\n%s", dc.Text)
	}
}

func TestBashOutputStaysQuietForShortResult(t *testing.T) {
	if dc := displayBashOutput(context.Background(), bashResultEvent("one\ntwo", "")); dc != nil {
		t.Errorf("short output needs no echo, got:\n%s", dc.Text)
	}
}

func TestBashOutputIncludesStderr(t *testing.T) {
	dc := displayBashOutput(context.Background(), bashResultEvent(longOutput(6), "a warning"))
	if dc == nil || !strings.Contains(dc.Text, "a warning") {
		t.Errorf("stderr must be echoed too, got %v", dc)
	}
}

func TestBashOutputIgnoresEmpty(t *testing.T) {
	if dc := displayBashOutput(context.Background(), bashResultEvent("", "")); dc != nil {
		t.Error("empty output should produce nothing")
	}
	if dc := displayBashOutput(context.Background(), bashResultEvent("   \n  \n  \n  \n  \n", "")); dc != nil {
		t.Error("whitespace-only output should produce nothing")
	}
}

func TestBashOutputIgnoresMalformedResponse(t *testing.T) {
	ev := &hook.Event{HookEventName: "PostToolUse", ToolName: "Bash", ToolResponse: []byte("not json")}
	if dc := displayBashOutput(context.Background(), ev); dc != nil {
		t.Error("malformed tool_response must fail open, not panic or echo")
	}
}

// enabled belongs to the framework, so this goes through the registry.
func TestBashOutputRespectsConfig(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = false")
	if dc := hook.RunDisplays(bashResultEvent(longOutput(50), "")); dc != nil {
		t.Error("enabled=false must silence the rule")
	}
}

// The Bash rule must not fire for other tools sharing the PostToolUse event.
func TestBashOutputScopedToBashTool(t *testing.T) {
	ev := bashResultEvent(longOutput(20), "")
	ev.ToolName = "Read"
	if dc := hook.RunDisplays(ev); dc != nil {
		t.Errorf("rule leaked onto tool %q", ev.ToolName)
	}
}

func TestBashCommandHeader(t *testing.T) {
	if got := bashCommand(nil); got != "" {
		t.Errorf("missing command must yield no header, got %q", got)
	}
	if got := bashCommand([]byte(`{"command":"  ls -la  "}`)); got != "$ ls -la" {
		t.Errorf("header should trim, got %q", got)
	}
}

func TestBashOutputColorsCommandAndBodyDifferently(t *testing.T) {
	dc := displayBashOutput(context.Background(), bashResultEvent(longOutput(8), ""))
	if dc == nil {
		t.Fatal("want an echo")
	}
	cmdSeq := hook.TrueColor(defaultConfig().CommandColor)
	outSeq := hook.TrueColor(defaultConfig().OutputColor)
	if cmdSeq == outSeq {
		t.Fatal("defaults must differ or there is nothing to see")
	}
	for _, want := range []string{cmdSeq, outSeq, hook.Reset} {
		if !strings.Contains(dc.Text, want) {
			t.Errorf("missing escape %q in:\n%q", want, dc.Text)
		}
	}
}

func TestBashOutputUncoloredWhenConfigured(t *testing.T) {
	hooktest.Config(t, RuleID, "command_color = \"\"\noutput_color = \"\"")
	dc := displayBashOutput(context.Background(), bashResultEvent(longOutput(8), ""))
	if dc == nil {
		t.Fatal("want an echo")
	}
	if strings.Contains(dc.Text, "\x1b[") {
		t.Errorf("empty colors must emit no escapes:\n%q", dc.Text)
	}
}

func TestTruecolorRejectsJunk(t *testing.T) {
	for _, bad := range []string{"", "xyz", "12345", "gggggg", "#12345"} {
		if got := hook.TrueColor(bad); got != "" {
			t.Errorf("hook.TrueColor(%q) should be empty, got %q", bad, got)
		}
	}
	if got := hook.TrueColor("#50fa7b"); got != "\x1b[38;2;80;250;123m" {
		t.Errorf("hex parse wrong: %q", got)
	}
}

func bashFailEvent(errText string) *hook.Event {
	in, _ := json.Marshal(map[string]any{"command": "false && seq 1 9"})
	return &hook.Event{
		HookEventName: "PostToolUseFailure",
		ToolName:      "Bash",
		ToolInput:     in,
		Error:         errText,
	}
}

// A non-zero exit fires PostToolUseFailure with no tool_response at all, so the
// body has to come from `error` or the echo silently disappears on failures.
func TestBashOutputEchoesFailureFromErrorField(t *testing.T) {
	dc := displayBashOutput(context.Background(), bashFailEvent("Exit code 3\n"+longOutput(8)))
	if dc == nil {
		t.Fatal("a failed command must still echo")
	}
	if !strings.Contains(dc.Text, "Exit code 3") {
		t.Errorf("exit code missing:\n%s", dc.Text)
	}
	if strings.Count(dc.Text, "line") != 8 {
		t.Errorf("failure output truncated:\n%s", dc.Text)
	}
}

func TestBashOutputPaintsFailedCommandRed(t *testing.T) {
	cfg := defaultConfig()
	ok := displayBashOutput(context.Background(), bashResultEvent(longOutput(8), ""))
	bad := displayBashOutput(context.Background(), bashFailEvent("Exit code 1\n"+longOutput(8)))
	if ok == nil || bad == nil {
		t.Fatal("want both echoes")
	}
	if !strings.Contains(ok.Text, hook.TrueColor(cfg.CommandColor)) {
		t.Error("success header should use the command colour")
	}
	if !strings.Contains(bad.Text, hook.TrueColor(cfg.FailedColor)) {
		t.Error("failure header should use the failed colour")
	}
	if strings.Contains(bad.Text, hook.TrueColor(cfg.CommandColor)) {
		t.Error("failure header must not also carry the success colour")
	}
}

func TestBashOutputIgnoresEmptyFailure(t *testing.T) {
	if dc := displayBashOutput(context.Background(), bashFailEvent("")); dc != nil {
		t.Error("an empty error should produce nothing")
	}
}
