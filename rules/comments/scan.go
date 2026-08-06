package comments

import "strings"

type commentKind int

const (
	lineComment commentKind = iota
	blockComment
	docstring
)

// scannedComment is one comment found in source. Consecutive single-line
// comments coalesce into one (lines holds every physical line, startLine the
// first). startByte/endByte are its byte span (widened over a coalesced run).
type scannedComment struct {
	startLine int
	startByte int
	endByte   int
	lines     []string
	kind      commentKind
	leading   bool // marker preceded only by whitespace on its line (not a trailing comment)
	// fileHeader marks a comment that sits above the package clause: godoc's
	// front page, or a licence header. Both are meant to be prose.
	fileHeader bool
}

// delimiter is an open/close token pair. For line comments close is "" (runs
// to end of line); for same-delimiter strings/docstrings open == close.
type delimiter struct {
	open, close string
}

// langSyntax describes how one language delimits comments and strings. Strings
// are tracked only so a comment token inside a literal ("http://x") is not
// mistaken for a comment. escapes marks string kinds where a backslash escapes
// the closing quote (Go/JS/Python double & single quotes; NOT raw/backtick).
type langSyntax struct {
	line   []string
	block  []delimiter
	doc    []delimiter // triple-quote docstrings (python); treated as comments
	strEsc []delimiter // strings with backslash escaping
	strRaw []delimiter // strings without escaping (raw/backtick/single-quote sh)
}

// syntaxByExt maps a lowercased extension to its syntax. Absent → not scanned.
// C/Rust/etc. get //+/* */ coverage for free by sharing the C-family entry.
var syntaxByExt = func() map[string]langSyntax {
	cFamily := langSyntax{
		line:   []string{"//"},
		block:  []delimiter{{"/*", "*/"}},
		strEsc: []delimiter{{`"`, `"`}, {"'", "'"}},
	}
	goSyntax := langSyntax{
		line:   []string{"//"},
		block:  []delimiter{{"/*", "*/"}},
		strEsc: []delimiter{{`"`, `"`}, {"'", "'"}},
		strRaw: []delimiter{{"`", "`"}},
	}
	jsSyntax := langSyntax{
		line:   []string{"//"},
		block:  []delimiter{{"/*", "*/"}},
		strEsc: []delimiter{{`"`, `"`}, {"'", "'"}},
		strRaw: []delimiter{{"`", "`"}},
	}
	py := langSyntax{
		line:   []string{"#"},
		doc:    []delimiter{{`"""`, `"""`}, {"'''", "'''"}},
		strEsc: []delimiter{{`"`, `"`}, {"'", "'"}},
	}
	sh := langSyntax{
		line:   []string{"#"},
		strEsc: []delimiter{{`"`, `"`}},
		strRaw: []delimiter{{"'", "'"}},
	}
	lua := langSyntax{
		line:   []string{"--"},
		block:  []delimiter{{"--[[", "]]"}},
		strEsc: []delimiter{{`"`, `"`}, {"'", "'"}},
		strRaw: []delimiter{{"[[", "]]"}},
	}
	// """ must precede " — matchDelim takes the first hit, so reversed, a
	// multiline literal's opener reads as an empty string and a // inside it
	// scans as a comment. Not cFamily: Swift has no single-quote strings.
	swift := langSyntax{
		line:   []string{"//"},
		block:  []delimiter{{"/*", "*/"}},
		strEsc: []delimiter{{`"""`, `"""`}, {`"`, `"`}},
	}
	hcl := langSyntax{
		line:   []string{"#", "//"},
		block:  []delimiter{{"/*", "*/"}},
		strEsc: []delimiter{{`"`, `"`}},
	}
	yaml := langSyntax{
		line:   []string{"#"},
		strEsc: []delimiter{{`"`, `"`}},
		strRaw: []delimiter{{"'", "'"}},
	}
	return map[string]langSyntax{
		".go":    goSyntax,
		".py":    py,
		".pyi":   py,
		".js":    jsSyntax,
		".jsx":   jsSyntax,
		".ts":    jsSyntax,
		".tsx":   jsSyntax,
		".mjs":   jsSyntax,
		".cjs":   jsSyntax,
		".lua":   lua,
		".sh":    sh,
		".bash":  sh,
		".zsh":   sh,
		".tf":    hcl,
		".tofu":  hcl,
		".yaml":  yaml,
		".yml":   yaml,
		".c":     cFamily,
		".h":     cFamily,
		".cpp":   cFamily,
		".cc":    cFamily,
		".hpp":   cFamily,
		".rs":    cFamily,
		".java":  cFamily,
		".swift": swift,
	}
}()

// scanComments finds every comment/docstring in src for the given syntax,
// coalescing runs of adjacent single-line comments into one block. Content
// inside string literals is skipped so a comment token in a string is ignored.
// precedesPackageClause reports whether the next code after off is a package
// clause. Blank lines are skipped so a licence header separated from the
// clause counts too — nagging about either is wrong.
// opensTheFile reports a comment with nothing but blank lines, a shebang or an
// encoding line before it. A Python module docstring is the same thing as a Go
// package doc — the file's front page, and prose by convention.
func opensTheFile(src string, start int) bool {
	for _, line := range strings.Split(src[:start], "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return false
	}
	return true
}

func precedesPackageClause(src string, off int) bool {
	if off >= len(src) {
		return false
	}
	for _, line := range strings.Split(src[off:], "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		return strings.HasPrefix(t, "package ")
	}
	return false
}

func scanComments(src string, syn langSyntax) []scannedComment {
	var out []scannedComment
	line := 1
	i := 0
	n := len(src)
	atLineStart := true // only whitespace seen since the last newline

	appendComment := func(startLine, startByte, endByte int, text []string, kind commentKind, leading bool) {
		// Coalesce a run of LEADING line comments on consecutive lines into one
		// block (for the >N-lines rule). Trailing comments (code before them)
		// never coalesce, so `x() // a` stays a 1-line comment.
		if kind == lineComment && leading && len(out) > 0 {
			prev := &out[len(out)-1]
			if prev.kind == lineComment && prev.leading && prev.startLine+len(prev.lines) == startLine {
				prev.lines = append(prev.lines, text...)
				prev.endByte = endByte
				return
			}
		}
		out = append(out, scannedComment{
			startLine: startLine, startByte: startByte, endByte: endByte,
			lines: text, kind: kind, leading: leading,
		})
	}

	for i < n {
		switch src[i] {
		case '\n':
			line++
			i++
			atLineStart = true
			continue
		case ' ', '\t', '\r':
			i++
			continue
		}
		leading := atLineStart
		atLineStart = false

		if d, ok := matchDelim(src, i, syn.doc); ok {
			text, ni, nl := consumeBlock(src, i, d, line)
			// Only a docstring when it opens the line. `sql = """SELECT ..."""`
			// is a string literal, and treating every triple quote as a comment
			// flagged multi-line SQL and templates as bloated prose.
			if leading {
				appendComment(line, i, ni, text, docstring, leading)
			}
			i, line = ni, nl
			continue
		}
		if d, ok := matchDelim(src, i, syn.block); ok {
			text, ni, nl := consumeBlock(src, i, d, line)
			appendComment(line, i, ni, text, blockComment, leading)
			i, line = ni, nl
			continue
		}
		if _, ok := matchToken(src, i, syn.line); ok {
			text, ni := consumeLine(src, i)
			appendComment(line, i, ni, []string{text}, lineComment, leading)
			i = ni
			continue
		}
		if d, ok := matchDelim(src, i, syn.strEsc); ok {
			i, line = skipString(src, i, d, true, line)
			continue
		}
		if d, ok := matchDelim(src, i, syn.strRaw); ok {
			i, line = skipString(src, i, d, false, line)
			continue
		}
		i++
	}
	for k := range out {
		// Either shape of file front page: a Go comment above the package
		// clause, or a module docstring opening the file.
		out[k].fileHeader = precedesPackageClause(src, out[k].endByte) ||
			(out[k].kind == docstring && opensTheFile(src, out[k].startByte))
	}
	return out
}

func matchToken(src string, i int, tokens []string) (string, bool) {
	for _, t := range tokens {
		if strings.HasPrefix(src[i:], t) {
			return t, true
		}
	}
	return "", false
}

func matchDelim(src string, i int, delims []delimiter) (delimiter, bool) {
	for _, d := range delims {
		if strings.HasPrefix(src[i:], d.open) {
			return d, true
		}
	}
	return delimiter{}, false
}

// consumeLine returns the rest of the current line and the index of its newline
// (or end of source).
func consumeLine(src string, i int) (string, int) {
	end := strings.IndexByte(src[i:], '\n')
	if end < 0 {
		return src[i:], len(src)
	}
	return src[i : i+end], i + end
}

// consumeBlock returns the text of a block/docstring (its own lines split on
// \n), the index just past the closing delimiter, and the updated line number.
// An unterminated block runs to end of source.
func consumeBlock(src string, i int, d delimiter, line int) ([]string, int, int) {
	rest := src[i+len(d.open):]
	closeAt := strings.Index(rest, d.close)
	var body string
	var end int
	if closeAt < 0 {
		body = src[i:]
		end = len(src)
	} else {
		end = i + len(d.open) + closeAt + len(d.close)
		body = src[i:end]
	}
	return strings.Split(body, "\n"), end, line + strings.Count(body, "\n")
}

// skipString advances past a string literal so comment tokens inside it are
// ignored. When esc is true a backslash escapes the closing quote. An
// unterminated string runs to end of source.
func skipString(src string, i int, d delimiter, esc bool, line int) (int, int) {
	j := i + len(d.open)
	for j < len(src) {
		if src[j] == '\n' {
			line++
			j++
			continue
		}
		if esc && src[j] == '\\' {
			j += 2
			continue
		}
		if strings.HasPrefix(src[j:], d.close) {
			return j + len(d.close), line
		}
		j++
	}
	return len(src), line
}

// maxLineLen returns the longest line in a comment, counted in runes. Bytes
// would make the budget shrink for anyone writing non-ASCII: an em-dash costs
// three, so a comment in Japanese would trip an "80 character" limit at 27.
func maxLineLen(c scannedComment) int {
	m := 0
	for _, l := range c.lines {
		if n := len([]rune(l)); n > m {
			m = n
		}
	}
	return m
}
