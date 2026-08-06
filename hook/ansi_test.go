package hook

import (
	"os"
	"testing"
)

// NO_COLOR is a standard, and hook output gets piped into logs where escapes
// are noise.
func TestNoColorSuppressesPaint(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if got := Paint("hello", "ff5555"); got != "hello" {
		t.Errorf("NO_COLOR set, want plain text, got %q", got)
	}
}

func TestPaintColoursWhenNoColorIsUnset(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	_ = os.Unsetenv("NO_COLOR")
	if got := Paint("hello", "ff5555"); got == "hello" {
		t.Error("want an escape sequence when NO_COLOR is unset")
	}
}

func TestStripRemovesEscapes(t *testing.T) {
	painted := Paint("hello", "ff5555")
	if got := Strip(painted); got != "hello" {
		t.Errorf("Strip(%q) = %q, want %q", painted, got, "hello")
	}
	if len([]rune(Strip(painted))) != 5 {
		t.Error("a stripped string must measure as its visible width")
	}
}
