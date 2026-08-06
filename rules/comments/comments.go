package comments

import (
	"context"
	"fmt"
	"strings"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// checkComments flags comments the change TOUCHED that are verbose (block over
// the line budget, or a line over the length budget) or read like agent memos.
// Untouched comments are never considered. Whitelisted markers (BDD, linter
// directives, shebangs) are exempt. Advisory PostToolUse — the edit already
// landed; findings tell the model to justify, slim, or remove.
func checkComments(ctx context.Context, ev *hook.Event) []hook.Finding {
	cfg := loadConfig()
	edited, ok := ev.EditedFile()
	if !ok {
		return nil
	}
	syn, ok := syntaxByExt[strings.ToLower(edited.Ext())]
	if !ok {
		return nil
	}

	// A Write has no Before, so newComments falls through to scanning the whole
	// file — which is correct: every comment in a new file is a touched one.
	var touched []scannedComment
	for _, edit := range edited.Edits {
		touched = append(touched, newComments(edit.Before, edit.After, syn)...)
	}

	type flagged struct {
		c      scannedComment
		reason string
	}
	var hits []flagged
	memo := false
	for _, c := range touched {
		if skipWhitelisted(c) {
			continue
		}
		reason, ok := triggerReason(c, cfg)
		if !ok {
			continue
		}
		if reason == reasonMemo {
			memo = true
		}
		hits = append(hits, flagged{c, reason})
	}
	if len(hits) == 0 {
		return nil
	}

	findings := make([]hook.Finding, 0, len(hits)+1)
	findings = append(findings, hook.Finding{Message: commentGuidance(memo)})
	for _, h := range hits {
		findings = append(findings, hook.Finding{
			Message: fmt.Sprintf("L%d [%s]: %s", h.c.startLine, h.reason, truncateComment(h.c)),
		})
	}
	return findings
}

// newComments returns comments present in newStr whose whole text is not
// already in oldStr — i.e. added or modified by the edit, evaluated as whole
// blocks. A comment that survives an edit unchanged is untouched and dropped.
func newComments(oldStr, newStr string, syn langSyntax) []scannedComment {
	fresh := scanComments(newStr, syn)
	if strings.TrimSpace(oldStr) == "" {
		return fresh
	}
	old := make(map[string]bool)
	for _, c := range scanComments(oldStr, syn) {
		old[normalizeComment(c)] = true
	}
	var out []scannedComment
	for _, c := range fresh {
		if !old[normalizeComment(c)] {
			out = append(out, c)
		}
	}
	return out
}

func normalizeComment(c scannedComment) string {
	trimmed := make([]string, len(c.lines))
	for i, l := range c.lines {
		trimmed[i] = strings.TrimSpace(l)
	}
	return strings.Join(trimmed, "\n")
}

const (
	reasonMemo = "agent-memo"
)

// triggerReason reports why a touched comment fires, or ok=false if it's fine.
func triggerReason(c scannedComment, cfg config) (string, bool) {
	if len(c.lines) > cfg.MaxCommentLines {
		return fmt.Sprintf(">%d lines", cfg.MaxCommentLines), true
	}
	if maxLineLen(c) > cfg.MaxLineLength {
		return fmt.Sprintf(">%d chars", cfg.MaxLineLength), true
	}
	if cfg.AgentMemo && isAgentMemo(c) {
		return reasonMemo, true
	}
	return "", false
}

func truncateComment(c scannedComment) string {
	return hook.Truncate(strings.TrimSpace(strings.Join(c.lines, " ")), 120)
}

// commentGuidance tells the model how to resolve each flagged comment. Newlines
// split into separate rows via expandFindings, one per rendered table line.
// Priority structure derived from comment-checker (MIT) — see NOTICE.
func commentGuidance(memo bool) string {
	base := "Comment gate: your change touched comment(s) that are verbose or read like agent memos " +
		"(see the lines below). For EACH one:\n" +
		"  (a) if it predates your change, keep it and say so;\n" +
		"  (b) if it's a BDD given/when/then marker, keep it;\n" +
		"  (c) if it genuinely earns its place — a non-obvious WHY, a gotcha that will bite the next " +
		"reader, or complex algorithm/security/perf/regex/math — keep it and justify in one line;\n" +
		"  (d) otherwise SLIM it to a single sharp line or delete it and let self-documenting code " +
		"(clear names, small functions) carry the meaning.\n" +
		"Prefer tightening a bloated comment over dumping a wall of prose OR stripping real signal. " +
		"While you're in this file, also slim any adjacent EXISTING comments that are needlessly verbose. " +
		"This standard applies to all future comments, not just this edit."
	if memo {
		return "AGENT-MEMO smell: one or more flagged comments narrate WHAT you changed (\"changed X to Y\", " +
			"\"now this...\", \"Note:\"). Those are cruft — git history is the record — so delete them outright.\n" +
			"Then: " + base
	}
	return base
}

// --- whitelist: word lists from comment-checker (MIT) — see NOTICE --------

var bddKeywords = []string{"given", "when", "then", "arrange", "act", "assert", "when & then", "when&then"}

var directivePrefixes = []string{
	"type:", "noqa", "pyright:", "ruff:", "mypy:", "pylint:", "flake8:", "pyre:", "pytype:",
	"eslint-disable", "eslint-ignore", "prettier-ignore", "ts-ignore", "ts-expect-error",
	"clippy:", "allow", "deny", "warn", "forbid",
}

// skipWhitelisted exempts a comment only when EVERY line is a whitelisted
// marker (BDD, linter directive, shebang) — so a lone `# given` or `//nolint`
// passes but a real multi-line comment never sneaks through.
func skipWhitelisted(c scannedComment) bool {
	// A package doc comment is godoc's rendered front page and Go convention
	// wants it to be prose. Flagging it would fire on every Go file written.
	if c.fileHeader {
		return true
	}
	for _, l := range c.lines {
		t := stripMarkers(l)
		if !isBDD(t) && !isDirective(t) && !strings.HasPrefix(strings.TrimSpace(l), "#!") {
			return false
		}
	}
	return len(c.lines) > 0
}

func isBDD(text string) bool {
	for _, k := range bddKeywords {
		if strings.EqualFold(text, k) {
			return true
		}
	}
	return false
}

func isDirective(text string) bool {
	t := strings.TrimPrefix(text, "@")
	t = strings.TrimSpace(t)
	for _, d := range directivePrefixes {
		if len(t) >= len(d) && strings.EqualFold(t[:len(d)], d) {
			return true
		}
	}
	return false
}

// --- agent-memo detection: vocabulary from comment-checker (MIT), see NOTICE

// memoPrefixes open a comment that narrates a change. Present-tense verbs
// ("update", "replace") are omitted: they collide with Go doc comments whose
// first word is the identifier, e.g. "Update refreshes the token".
var memoPrefixes = []string{
	"added ", "after this", "before this",
	"changed ", "converted ",
	"deleted ", "here we ",
	"implemented ", "implementation note",
	"modified ", "moved ", "migrated ",
	"now we ", "now this ", "now it ", "note:",
	"previously ", "refactored ", "replaced ", "removed ",
	"renamed ", "switched ",
	"updated ", "was changed",
	"this implements", "this adds", "this removes", "this changes", "this fixes",
}

// memoPhrases are agent tells that land anywhere in a line, not just at the
// start — "load-bearing" is AI-comment boilerplate wherever it sits.
var memoPhrases = []string{"load-bearing", "load bearing"}

func isAgentMemo(c scannedComment) bool {
	for _, l := range c.lines {
		t := strings.ToLower(stripMarkers(l))
		for _, p := range memoPrefixes {
			if strings.HasPrefix(t, p) {
				return true
			}
		}
		for _, p := range memoPhrases {
			if strings.Contains(t, p) {
				return true
			}
		}
		if hasBareArrow(t) {
			return true
		}
	}
	return false
}

// hasBareArrow matches an "x -> y" memo shape where both sides start with a
// letter (e.g. "int -> string"), the pattern agents use to narrate a change.
func hasBareArrow(text string) bool {
	left, right, found := strings.Cut(text, "->")
	if !found {
		return false
	}
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	return left != "" && right != "" && isAlpha(rune(left[0])) && isAlpha(rune(right[0]))
}

func isAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// stripMarkers removes leading comment punctuation so whitelist/memo checks see
// the bare text.
func stripMarkers(line string) string {
	t := strings.TrimSpace(line)
	for _, m := range []string{"<!--", "///", "//", "#!", "#", "--[[", "--", "/**", "/*", "*/", "*", `"""`, "'''"} {
		if after, ok := strings.CutPrefix(t, m); ok {
			t = strings.TrimSpace(after)
			break
		}
	}
	return t
}
