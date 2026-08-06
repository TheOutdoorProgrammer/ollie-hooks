package hook

import (
	"strconv"
	"strings"
)

// defaultWrapWidth bounds a finding row. Hooks run with no controlling
// terminal, so the real width cannot be detected — it has to be a setting.
const defaultWrapWidth = 100

// continuationMarker is the rule-column glyph for a wrapped line, indented so
// the arrow sits under the last char of the rule label above. len-2, not len-1:
// TOON quotes the leading-space value, and that opening quote eats one column.
func continuationMarker(rule string) string {
	return strings.Repeat(" ", max(len([]rune(rule))-2, 0)) + "↳"
}

// expandFindings turns each finding into the rows TOON's 2-column table wants.
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
		marker := continuationMarker(f.Rule)
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
				out = append(out, Finding{Rule: marker, Message: line})
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
			Message: strconv.Itoa(len(findings)-n) + " more finding(s) omitted to fit the output limit",
		})
		out, err = encode(trimmed)
		if err != nil {
			return findings, "", err
		}
		if len(out) <= maxOutputChars {
			return trimmed, out, nil
		}
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
	)
	for _, w := range words {
		switch {
		case cur.Len() == 0:
			cur.WriteString(w)
		case cur.Len()+1+len([]rune(w)) <= width:
			cur.WriteString(" " + w)
		default:
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(w)
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}
