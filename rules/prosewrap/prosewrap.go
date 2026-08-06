package prosewrap

import (
	"context"
	"fmt"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// checkProseWrap flags markdown prose the change TOUCHED that is hard-wrapped
// mid-sentence. Untouched paragraphs are never considered. Advisory
// PostToolUse — the edit already landed; the finding asks for a reflow.
func checkProseWrap(ctx context.Context, ev *hook.Event) []hook.Finding {
	cfg := loadConfig()
	edited, ok := ev.EditedFile()
	if !ok || !cfg.applies(edited.Path) {
		return nil
	}

	var wraps, dashes []proseWrap
	for _, text := range edited.Added() {
		wraps = append(wraps, scanProseWraps(text, cfg.MaxReported)...)
		if cfg.FlagEmDashes {
			dashes = append(dashes, scanEmDashes(text, cfg.MaxReported)...)
		}
	}
	if len(wraps) == 0 && len(dashes) == 0 {
		return nil
	}

	findings := make([]hook.Finding, 0, len(wraps)+len(dashes)+2)
	findings = append(findings, section(proseGuidance(), wraps)...)
	findings = append(findings, section(emDashGuidance(), dashes)...)
	return findings
}

// section turns a batch of flagged lines into a guidance finding plus one line
// each, or nothing when the batch is empty.
func section(guidance string, lines []proseWrap) []hook.Finding {
	if len(lines) == 0 {
		return nil
	}
	out := make([]hook.Finding, 0, len(lines)+1)
	out = append(out, hook.Finding{Message: guidance})
	for _, w := range lines {
		out = append(out, hook.Finding{Message: fmt.Sprintf("L%d: %s", w.line, truncateProse(w.text))})
	}
	return out
}

func emDashGuidance() string {
	return `
		Em-dash gate: these lines use an em-dash (—). Replace each with a comma,
		colon, period, or parentheses, whichever the sentence wants. Opt-in house
		style: set flag_em_dashes = false under [rules.prose-wrap] to turn it off.`
}

func proseGuidance() string {
	return `
		Prose gate: these lines break a sentence across lines. Markdown joins
		adjacent lines into one paragraph, so hand-wrapping changes nothing that
		renders and costs real money in diffs — reword one sentence and every
		line after it reflows, burying the actual change. Put each sentence on
		its own line and let the renderer wrap it. Blank lines still separate
		paragraphs; tables, code blocks and deliberate two-space breaks are
		exempt. Fix the lines below by joining each wrapped sentence onto one
		line, and leave surrounding sentences alone.`
}

func truncateProse(text string) string {
	return hook.Truncate(text, 90)
}
