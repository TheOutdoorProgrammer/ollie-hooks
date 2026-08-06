package prosewrap

import (
	"strings"
	"testing"
)

func TestScanProseWraps(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{
			"a sentence broken across lines",
			"This is a sentence that someone\nwrapped by hand.\n",
			1,
		},
		{
			"one sentence per line",
			"This is one sentence.\nAnd this is another.\n",
			0,
		},
		// Long lines are the POINT of this rule, not a violation of it. The
		// 80-char budget belongs to the comments rule, which never scans
		// markdown, and markdownlint's MD013 is off globally for the same reason.
		{
			"a single sentence well over 100 characters",
			"This one sentence runs to well over one hundred characters on a single line, which is exactly what the rule is asking for rather than something it should flag.\n",
			0,
		},
		{
			"a very long sentence and a second one, each on its own line",
			strings.Repeat("word ", 60) + "end.\n" + strings.Repeat("more ", 60) + "done.\n",
			0,
		},
		// Everything below is a line markdown breaks for its own reasons, and
		// the scanner must not read any of them as wrapped prose.
		{
			"table rows",
			"| Scope | What happens |\n|---|---|\n| Release | tags and publishes |\n",
			0,
		},
		{
			"front matter",
			"---\nname: a-thing\ndescription: what it does\n---\n\nBody.\n",
			0,
		},
		{
			"fenced code",
			"Text.\n\n```sh\nexport FOO=bar\nrun --thing\n```\n",
			0,
		},
		{
			"tilde fenced code",
			"Text.\n\n~~~\nexport FOO=bar\nrun --thing\n~~~\n",
			0,
		},
		{
			"indented code block",
			"Text.\n\n    export FOO=bar\n    run --thing\n",
			0,
		},
		{
			"heading above lowercase prose",
			"# Title\nsome body text.\n",
			0,
		},
		{
			"a two-space hard break is deliberate",
			"First half  \nsecond half.\n",
			0,
		},
		{
			"a trailing backslash is deliberate",
			"First half\\\nsecond half.\n",
			0,
		},
		{
			"a reference definition",
			"[label]: https://example.com\nsome text.\n",
			0,
		},
		{
			"html block",
			"<div>\nsomething\n</div>\n",
			0,
		},
		{
			"a wrapped list item",
			"- **Thing** does something,\n  and then something else.\n",
			1,
		},
		{
			"list items that are not wrapped",
			"- apples\n- oranges\n- pears\n",
			0,
		},
		// Conservative by design: a missing full stop reads as two lines, not a
		// wrap, because the continuation is capitalised.
		{
			"next line starts uppercase",
			"This has no full stop\nAnd this starts a new one.\n",
			0,
		},
		{
			"several wraps in one paragraph",
			"one line that keeps\ngoing and then keeps\ngoing again.\n",
			2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := scanProseWraps(c.text, maxProseWraps)
			if len(got) != c.want {
				t.Fatalf("scanProseWraps found %d wraps %v, want %d", len(got), got, c.want)
			}
		})
	}
}

func TestScanProseWrapsReportsTheLine(t *testing.T) {
	got := scanProseWraps("Fine sentence.\n\nthis one wraps\nonto here.\n", maxProseWraps)
	if len(got) != 1 {
		t.Fatalf("want 1 wrap, got %d (%v)", len(got), got)
	}
	if got[0].line != 3 {
		t.Errorf("line = %d, want 3", got[0].line)
	}
	if got[0].text != "this one wraps" {
		t.Errorf("text = %q, want %q", got[0].text, "this one wraps")
	}
}

func TestScanProseWrapsCaps(t *testing.T) {
	var b strings.Builder
	for range maxProseWraps * 3 {
		b.WriteString("a line that keeps\n")
	}
	if got := scanProseWraps(b.String(), maxProseWraps); len(got) != maxProseWraps {
		t.Fatalf("got %d wraps, want the cap of %d", len(got), maxProseWraps)
	}
}
