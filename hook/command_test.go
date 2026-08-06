package hook

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	cases := map[string][]string{
		"python3 policy.py":            {"python3", "policy.py"},
		"  spaced   out  ":             {"spaced", "out"},
		`python3 "my rules/policy.py"`: {"python3", "my rules/policy.py"},
		`python3 'my rules/policy.py'`: {"python3", "my rules/policy.py"},
		`prog --flag="a b" tail`:       {"prog", "--flag=a b", "tail"},
		`prog ""`:                      {"prog", ""},
		"":                             nil,
	}
	for in, want := range cases {
		got, err := splitCommand(in)
		if err != nil {
			t.Errorf("splitCommand(%q): %v", in, err)
			continue
		}
		if !slices.Equal(got, want) {
			t.Errorf("splitCommand(%q) = %#v, want %#v", in, got, want)
		}
	}
}

func TestSplitCommandRejectsAnUnbalancedQuote(t *testing.T) {
	if _, err := splitCommand(`prog "unterminated`); err == nil {
		t.Error("an unbalanced quote must be an error, not a silently mangled command")
	}
}

// The README's own quickstart uses a tilde. strings.Fields left it literal, so
// the example failed, and failed open.
func TestSplitCommandExpandsATilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	got, err := splitCommand("python3 ~/rules/policy.py")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "rules", "policy.py")
	if len(got) != 2 || got[1] != want {
		t.Errorf("got %#v, want second field %q", got, want)
	}
}

func TestExpandHomeLeavesOtherTildesAlone(t *testing.T) {
	for _, in := range []string{"~user/thing", "a~b", "./~/x", "prog~"} {
		if got := expandHome(in); got != in {
			t.Errorf("expandHome(%q) = %q, want it untouched", in, got)
		}
	}
}
