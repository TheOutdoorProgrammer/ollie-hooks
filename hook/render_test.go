package hook

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Byte slicing lands mid-character on any non-ASCII text, so the finding
// carried invalid UTF-8 that renders as replacement glyphs.
func TestTruncateKeepsValidUTF8(t *testing.T) {
	s := strings.Repeat("é", 200) // two bytes per rune
	got := Truncate(s, 50)

	if !utf8.ValidString(got) {
		t.Error("truncation produced invalid UTF-8")
	}
	if n := utf8.RuneCountInString(got); n != 50 {
		t.Errorf("got %d runes, want 50", n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("a cut string should say so: %q", got)
	}
}

func TestTruncateLeavesShortTextAlone(t *testing.T) {
	for _, s := range []string{"", "short", "exactly ten"} {
		if got := Truncate(s, 20); got != s {
			t.Errorf("Truncate(%q) = %q, want it untouched", s, got)
		}
	}
}

// width is a column budget, so a multibyte line must be allowed to use all of
// it. Comparing byte length against it wrapped CJK roughly 3x too narrow.
func TestWrapTextCountsRunesNotBytes(t *testing.T) {
	// Ten 3-byte runes as separate words: 10 runes + 9 spaces = 19 columns.
	words := make([]string, 10)
	for i := range words {
		words[i] = "世"
	}
	got := wrapText(strings.Join(words, " "), 20)
	if len(got) != 1 {
		t.Errorf("19 columns should fit in a 20-column budget, got %d lines: %q", len(got), got)
	}
}
