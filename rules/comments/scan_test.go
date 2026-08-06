package comments

import "testing"

func TestScanCommentsGo(t *testing.T) {
	src := "package x\n" +
		"\n" +
		"// one\n" +
		"// two\n" +
		"s := \"a // b\" // trail\n" +
		"/* blk\n" +
		"   two */\n"
	got := scanComments(src, syntaxByExt[".go"])
	if len(got) != 3 {
		t.Fatalf("got %d comments, want 3: %+v", len(got), got)
	}
	if got[0].kind != lineComment || len(got[0].lines) != 2 || got[0].startLine != 3 || !got[0].leading {
		t.Errorf("block: %+v", got[0])
	}
	if got[1].kind != lineComment || len(got[1].lines) != 1 || got[1].startLine != 5 || got[1].leading {
		t.Errorf("trailing comment should be a lone non-leading line comment: %+v", got[1])
	}
	if got[2].kind != blockComment || len(got[2].lines) != 2 || got[2].startLine != 6 {
		t.Errorf("block comment: %+v", got[2])
	}
}

func TestScanCommentsStringSkip(t *testing.T) {
	// `//` and `#` inside string literals are not comments.
	for _, c := range []struct {
		ext, src string
	}{
		{".go", "url := \"http://example.com\"\n"},
		{".py", "u = 'a # b'\n"},
		{".go", "s := `raw // not a comment`\n"},
	} {
		if got := scanComments(c.src, syntaxByExt[c.ext]); len(got) != 0 {
			t.Errorf("%s: expected no comments in %q, got %+v", c.ext, c.src, got)
		}
	}
}

func TestScanCommentsPython(t *testing.T) {
	src := "def f():\n" +
		"    \"\"\"doc\n" +
		"    line2\"\"\"\n" +
		"    x = 1  # set\n" +
		"    # lead\n"
	got := scanComments(src, syntaxByExt[".py"])
	if len(got) != 3 {
		t.Fatalf("got %d, want 3: %+v", len(got), got)
	}
	if got[0].kind != docstring || len(got[0].lines) != 2 || got[0].startLine != 2 {
		t.Errorf("docstring: %+v", got[0])
	}
	if !got[2].leading || got[1].leading {
		t.Errorf("expected trailing '# set' then leading '# lead': %+v %+v", got[1], got[2])
	}
}

func TestScanCommentsCoalesce(t *testing.T) {
	src := "// a\n// b\n// c\n// d\nfunc(){}\n"
	got := scanComments(src, syntaxByExt[".go"])
	if len(got) != 1 || len(got[0].lines) != 4 {
		t.Fatalf("four consecutive leading // should be one 4-line block: %+v", got)
	}
}

func TestScanCommentsSwift(t *testing.T) {
	src := "import Foundation\n" +
		"/// doc\n" +
		"func f() {\n" +
		"    let s = \"a // b\"\n" +
		"    let m = \"\"\"\n" +
		"        // not a comment\n" +
		"        \"\"\"\n" +
		"    g() // trail\n" +
		"}\n"
	got := scanComments(src, syntaxByExt[".swift"])
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 (/// doc and // trail): %+v", len(got), got)
	}
	if got[0].kind != lineComment || got[0].startLine != 2 || !got[0].leading {
		t.Errorf("/// should scan as a leading line comment: %+v", got[0])
	}
	if got[1].startLine != 8 || got[1].leading {
		t.Errorf("want the trailing comment on L8, got: %+v", got[1])
	}
}

func TestScanCommentsLua(t *testing.T) {
	src := "-- line\nx = 1 -- trail\n--[[ block\n  two ]]\n"
	got := scanComments(src, syntaxByExt[".lua"])
	if len(got) != 3 {
		t.Fatalf("got %d, want 3: %+v", len(got), got)
	}
	if got[2].kind != blockComment || len(got[2].lines) != 2 {
		t.Errorf("lua block: %+v", got[2])
	}
}

// The budget is characters, not bytes. An em-dash costs three bytes, so byte
// counting would shrink an "80 character" limit to 27 for a CJK comment.
func TestMaxLineLenCountsRunesNotBytes(t *testing.T) {
	line := "a line with two em-dashes — and — otherwise plain ascii text here"
	c := scannedComment{lines: []string{line}}
	if got, want := maxLineLen(c), len([]rune(line)); got != want {
		t.Errorf("maxLineLen = %d, want %d runes (bytes would be %d)", got, want, len(line))
	}

	cjk := scannedComment{lines: []string{"日本語のコメント"}}
	if got := maxLineLen(cjk); got != 8 {
		t.Errorf("8 CJK characters must measure 8, got %d (bytes would be 24)", got)
	}
}

// A package doc comment is godoc's front page; Go convention wants it long.
// Flagging it would fire on every Go file anyone writes.
func TestPackageDocIsExempt(t *testing.T) {
	src := `// Package thing does a great many things, described here at length across
// several lines because that is exactly what a package doc comment is for and
// godoc renders it as the front page of the API.
package thing

// this one is a normal comment that runs on and on and on and should still be
// flagged because it is not attached to the package clause at all, no sir
func F() {}
`
	got := scanComments(src, syntaxByExt[".go"])
	if len(got) < 2 {
		t.Fatalf("want both comments scanned, got %d", len(got))
	}
	if !got[0].fileHeader {
		t.Error("the comment above the package clause must be marked as a header")
	}
	if got[1].fileHeader {
		t.Error("a comment inside the file must not be treated as a header")
	}
	if !skipWhitelisted(got[0]) {
		t.Error("a package doc comment must be exempt from the gate")
	}
	if skipWhitelisted(got[1]) {
		t.Error("an ordinary long comment must still be checked")
	}
}

// A licence header separated from the clause by a blank line deserves the same
// exemption — nagging about either is wrong.
func TestLicenceHeaderIsExempt(t *testing.T) {
	src := `// Copyright the authors. Licensed under the MIT License, the full text of
// which is reproduced in the LICENSE file at the root of this repository.

package thing
`
	got := scanComments(src, syntaxByExt[".go"])
	if len(got) != 1 || !got[0].fileHeader {
		t.Fatalf("a header above a blank line and the clause must be exempt, got %+v", got)
	}
}

// Nothing outside Go has a package clause, so the exemption must not leak.
func TestNonGoCommentsAreNotHeaders(t *testing.T) {
	src := "# a long shell comment that goes on for a while and should be checked\necho hi\n"
	got := scanComments(src, syntaxByExt[".sh"])
	if len(got) != 1 {
		t.Fatalf("want one comment, got %d", len(got))
	}
	if got[0].fileHeader {
		t.Error("a shell comment cannot be a package header")
	}
}
