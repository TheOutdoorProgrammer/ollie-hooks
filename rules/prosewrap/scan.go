package prosewrap

import (
	"strings"
	"unicode"
)

// proseWrap is one line broken mid-sentence.
type proseWrap struct {
	line int
	text string
}

// Longer than this and the finding list stops being readable.
const maxProseWraps = 10

// scanProseWraps finds prose lines that end mid-sentence and continue on the
// next line. Conservative on purpose: it fires only when the next line starts
// lowercase, which a heading, list item, table row or badge never does.
func scanProseWraps(text string, limit int) []proseWrap {
	lines := strings.Split(text, "\n")

	var out []proseWrap
	fence := ""
	inFrontMatter := false

	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)

		// Front matter is key: value pairs, which look exactly like wrapped prose.
		if i == 0 && trimmed == "---" {
			inFrontMatter = true
			continue
		}
		if inFrontMatter {
			if trimmed == "---" {
				inFrontMatter = false
			}
			continue
		}

		if token := fenceToken(trimmed); token != "" {
			switch {
			case fence == "":
				fence = token
			case strings.HasPrefix(token, fence):
				fence = ""
			}
			continue
		}
		if fence != "" || i+1 >= len(lines) {
			continue
		}

		if !isProseLine(raw, trimmed) || endsSentence(raw, trimmed) {
			continue
		}
		if !continuesSentence(strings.TrimSpace(lines[i+1])) {
			continue
		}

		out = append(out, proseWrap{line: i + 1, text: trimmed})
		if len(out) == limit {
			break
		}
	}
	return out
}

// scanEmDashes finds touched lines that use an em-dash, skipping fenced code
// and front matter where one may be deliberate. Opt-in via flag_em_dashes.
func scanEmDashes(text string, limit int) []proseWrap {
	lines := strings.Split(text, "\n")
	var out []proseWrap
	fence := ""
	inFrontMatter := false
	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if i == 0 && trimmed == "---" {
			inFrontMatter = true
			continue
		}
		if inFrontMatter {
			if trimmed == "---" {
				inFrontMatter = false
			}
			continue
		}
		if token := fenceToken(trimmed); token != "" {
			if fence == "" {
				fence = token
			} else if strings.HasPrefix(token, fence) {
				fence = ""
			}
			continue
		}
		if fence == "" && strings.ContainsRune(raw, '—') {
			out = append(out, proseWrap{line: i + 1, text: trimmed})
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

func fenceToken(trimmed string) string {
	for _, token := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, token) {
			return token
		}
	}
	return ""
}

// isProseLine excludes what markdown wraps for its own reasons: table rows,
// headings, HTML, link definitions, and anything indented like a code block.
func isProseLine(raw, trimmed string) bool {
	if trimmed == "" {
		return false
	}
	if indent(raw) >= 4 {
		return false
	}
	switch trimmed[0] {
	case '|', '#', '<', '=':
		return false
	}
	if strings.HasPrefix(trimmed, "---") {
		return false
	}
	// A reference definition ([label]: url) is a whole line by construction.
	if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]:") {
		return false
	}
	return true
}

// endsSentence also treats markdown's explicit line breaks as intentional:
// a trailing backslash, or the two-space break.
func endsSentence(raw, trimmed string) bool {
	if strings.HasSuffix(raw, "  ") || strings.HasSuffix(trimmed, "\\") {
		return true
	}
	if strings.HasSuffix(trimmed, "|") {
		return true
	}
	last := []rune(trimmed)[len([]rune(trimmed))-1]
	return strings.ContainsRune(".!?:;", last)
}

// continuesSentence is the precision knob: a wrapped sentence resumes in
// lowercase, so anything else is treated as a deliberate new line.
func continuesSentence(next string) bool {
	if next == "" {
		return false
	}
	return unicode.IsLower([]rune(next)[0])
}

func indent(raw string) int {
	return len(raw) - len(strings.TrimLeft(raw, " \t"))
}
