package mermaid

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func requireMermaidASCII(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(defaultConfig().Binary); err != nil {
		t.Skip("mermaid-ascii not installed")
	}
}

func TestRenderMermaidProducesASCII(t *testing.T) {
	requireMermaidASCII(t)
	art := renderMermaid([]string{"graph LR", "  A[start] --> B[end]"}, defaultConfig())
	if art == "" {
		t.Fatal("want a render, got empty")
	}
	for _, want := range []string{"start", "end", "┌"} {
		if !strings.Contains(art, want) {
			t.Errorf("render missing %q:\n%s", want, art)
		}
	}
}

func TestRenderMermaidSkipsUnsupportedDiagram(t *testing.T) {
	requireMermaidASCII(t)
	if art := renderMermaid([]string{"stateDiagram-v2", "  [*] --> Idle"}, defaultConfig()); art != "" {
		t.Errorf("unsupported diagram should yield nothing, got:\n%s", art)
	}
}

func TestRenderMermaidSkipsOverWideDiagram(t *testing.T) {
	requireMermaidASCII(t)
	cfg := defaultConfig()
	cfg.WidthCap = 5
	if art := renderMermaid([]string{"graph LR", "  A[wide] --> B[wider]"}, cfg); art != "" {
		t.Errorf("over-cap render should be dropped, got:\n%s", art)
	}
}

func TestRenderMermaidIgnoresEmptyBody(t *testing.T) {
	if art := renderMermaid([]string{"", "   "}, defaultConfig()); art != "" {
		t.Errorf("empty body should yield nothing, got %q", art)
	}
}

func TestRenderMermaidSurvivesMissingBinary(t *testing.T) {
	cfg := defaultConfig()
	cfg.Binary = "definitely-not-a-real-binary-xyzzy"
	if art := renderMermaid([]string{"graph LR", "  A-->B"}, cfg); art != "" {
		t.Errorf("a missing binary must degrade to empty, got %q", art)
	}
}

func TestStripClassDefRemovesStylingOnly(t *testing.T) {
	got := stripClassDef([]string{
		"graph LR",
		"  A[Alpha]:::hot --> B[Beta]",
		"  classDef hot fill:#ff5555",
	})
	if strings.Contains(got, "classDef") || strings.Contains(got, ":::") {
		t.Errorf("styling survived: %q", got)
	}
	if !strings.Contains(got, "A[Alpha]") {
		t.Errorf("node label lost: %q", got)
	}
}

// enabled belongs to the framework, so this has to go through the registry —
// calling the rule directly would bypass the thing under test.
func TestMermaidRespectsDisabled(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = false")
	ev := hooktest.MessageDisplay("s", "m", "```mermaid\ngraph LR\n A-->B\n```\n", true)
	if dc := hook.RunDisplays(ev); dc != nil {
		t.Errorf("enabled=false must disable the rule, got %q", dc.Text)
	}
}
