package mermaid

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const (
	mermaidFenceOpen  = "```mermaid"
	mermaidFenceClose = "```"
)

// renderMermaid returns ASCII for a fence body, or "" when the render can't be
// trusted: unsupported diagram type, wider than WidthCap, or no binary.
func renderMermaid(ctx context.Context, body []string, cfg config) string {
	if strings.TrimSpace(strings.Join(body, "\n")) == "" {
		return ""
	}
	// CommandContext, so an abandoned rule takes its renderer with it. This
	// fires once per streamed delta, and orphans would pile up fast.
	cmd := exec.CommandContext(ctx, cfg.Binary, "-f", "-")
	cmd.WaitDelay = time.Second
	cmd.Stdin = strings.NewReader(stripClassDef(body))
	raw, err := cmd.Output()
	if err != nil {
		return ""
	}
	art := strings.TrimRight(string(raw), "\n")
	if art == "" {
		return ""
	}
	for line := range strings.SplitSeq(art, "\n") {
		// Runes, not bytes: box-drawing glyphs are 3 bytes each, so len()
		// overstates width ~3x and would reject renders that fit fine.
		if len([]rune(line)) > cfg.WidthCap {
			return ""
		}
	}
	return art
}

// stripClassDef drops mermaid styling that ASCII cannot carry. A classDef line
// renders as a stray orphan box and pads every :::tagged node to match it.
func stripClassDef(body []string) string {
	kept := make([]string, 0, len(body))
	for _, line := range body {
		if strings.HasPrefix(strings.TrimSpace(line), "classDef") {
			continue
		}
		if i := strings.Index(line, ":::"); i >= 0 {
			line = line[:i]
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
