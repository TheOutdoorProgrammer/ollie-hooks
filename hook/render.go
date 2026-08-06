package hook

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// defaultWrapWidth bounds a finding row. Hooks run with no controlling
// terminal, so the real width cannot be detected — it has to be a setting.
const defaultWrapWidth = 100

// continuationGlyph marks a wrapped line's rule column. renderFindings
// right-aligns the column, so the glyph lands just before the divider under the
// rule label above without any manual indentation.
const continuationGlyph = "↳"

// expandFindings turns each finding into the rows the 2-column table wants.
//
// A message is written however reads best in source — indented raw strings
// included — and normalised here: whitespace inside a line collapses, so source
// indentation never reaches the screen, and an explicit newline stays a break.
// Rules never pre-flatten their own text; doing that by hand is what produced
// single lines far wider than the configured budget.
func expandFindings(findings []Finding, width int) []Finding {
	if width <= 0 {
		width = defaultWrapWidth
	}
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		first := true
		for _, para := range strings.Split(f.Message, "\n") {
			para = strings.Join(strings.Fields(para), " ")
			if para == "" {
				continue
			}
			for _, line := range wrapText(para, width) {
				if first {
					out = append(out, Finding{Rule: f.Rule, Message: line})
					first = false
					continue
				}
				out = append(out, Finding{Rule: continuationGlyph, Message: line})
			}
		}
		// A message that was entirely whitespace still needs a row, or the
		// finding vanishes and the block reason names a rule with no text.
		if first {
			out = append(out, Finding{Rule: f.Rule, Message: ""})
		}
	}
	return out
}

// renderFindings draws expanded findings as an aligned two-column table:
// the rule label right-aligned so dividers line up, then the message. It is
// display-only and renders without escaping — never parse the output.
func renderFindings(findings []Finding) string {
	width := 0
	for _, f := range findings {
		if n := utf8.RuneCountInString(f.Rule); n > width {
			width = n
		}
	}
	var b strings.Builder
	for i, f := range findings {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(" ", width-utf8.RuneCountInString(f.Rule)))
		b.WriteString(f.Rule)
		b.WriteString(" │ ")
		b.WriteString(f.Message)
	}
	return b.String()
}

// maxOutputChars is Claude Code's cap on hook output. Past it the content is
// replaced by a FILE PATH plus a preview, so an oversized findings table
// reaches the model as a filename rather than as findings.
const maxOutputChars = 10000

// fitFindings drops trailing findings until the encoded reason fits, replacing
// them with a row saying how many were lost. Encoding is retried rather than
// estimated because TOON's own quoting affects the length.
func fitFindings(findings []Finding, encode func([]Finding) (string, error)) ([]Finding, string, error) {
	out, err := encode(findings)
	if err != nil || len(out) <= maxOutputChars {
		return findings, out, err
	}
	for n := len(findings) - 1; n > 0; n-- {
		trimmed := make([]Finding, n, n+1)
		copy(trimmed, findings[:n])
		trimmed = append(trimmed, Finding{
			Rule:    "…",
			Message: strconv.Itoa(len(findings)-n) + " more line(s) omitted to fit the output limit",
		})
		out, err = encode(trimmed)
		if err != nil {
			return findings, "", err
		}
		if len(out) <= maxOutputChars {
			return trimmed, out, nil
		}
	}
	// Last resort: one finding whose message is a single unsplittable word past
	// the cap. Binary-search the longest truncation whose encoding fits, so the
	// reason stays findings instead of degrading to a file path.
	omitted := len(findings) - 1
	build := func(runes int) []Finding {
		fs := []Finding{{Rule: findings[0].Rule, Message: Truncate(findings[0].Message, runes)}}
		if omitted > 0 {
			fs = append(fs, Finding{
				Rule:    "…",
				Message: strconv.Itoa(omitted) + " more line(s) omitted to fit the output limit",
			})
		}
		return fs
	}
	lo, hi := 1, utf8.RuneCountInString(findings[0].Message)
	var fitted []Finding
	for lo <= hi {
		mid := (lo + hi) / 2
		cand := build(mid)
		enc, encErr := encode(cand)
		if encErr != nil {
			return findings, "", encErr
		}
		if len(enc) <= maxOutputChars {
			fitted, out, err = cand, enc, nil
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if fitted != nil {
		return fitted, out, nil
	}
	return findings, out, err
}

// NormalizeProse collapses a raw string's source indentation, keeping blank-line
// breaks. Findings get this from the TOON encoder; prose bound for
// additionalContext needs it too, or Go indentation reaches the model.
func NormalizeProse(s string) string {
	paras := strings.Split(s, "\n\n")
	out := make([]string, 0, len(paras))
	for _, p := range paras {
		if flat := strings.Join(strings.Fields(p), " "); flat != "" {
			out = append(out, flat)
		}
	}
	return strings.Join(out, "\n\n")
}

// wrapText breaks a single normalised line at word boundaries. A word longer
// than the width is left intact rather than split: a URL or a path chopped in
// half is worse than one long row.
func wrapText(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var (
		lines []string
		cur   strings.Builder
		// Counted in runes, because width is a column budget. Builder.Len is
		// bytes, so mixing the two wrapped multibyte text about three times
		// narrower than asked.
		n int
	)
	for _, w := range words {
		wl := utf8.RuneCountInString(w)
		switch {
		case n == 0:
			cur.WriteString(w)
			n = wl
		case n+1+wl <= width:
			cur.WriteString(" " + w)
			n += 1 + wl
		default:
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(w)
			n = wl
		}
	}
	if n > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// Truncate shortens s to at most n runes, marking the cut with an ellipsis.
// Runes rather than bytes: slicing a byte index lands mid-character on any
// non-ASCII text and puts invalid UTF-8 into the finding.
func Truncate(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
