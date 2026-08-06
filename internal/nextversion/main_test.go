package main

import "testing"

func TestCompute(t *testing.T) {
	cases := []struct {
		name      string
		tags      []string
		bump      string
		candidate bool
		want      string
	}{
		{"first release", nil, "patch", false, "v0.1.0"},
		{"first candidate", nil, "patch", true, "v0.1.0-rc.1"},

		{"patch", []string{"v1.1.0", "v1.2.0"}, "patch", false, "v1.2.1"},
		{"minor", []string{"v1.1.0", "v1.2.0"}, "minor", false, "v1.3.0"},
		{"major", []string{"v1.1.0", "v1.2.0"}, "major", false, "v2.0.0"},

		{"major resets minor and patch", []string{"v1.4.7"}, "major", false, "v2.0.0"},
		{"minor resets patch", []string{"v1.4.7"}, "minor", false, "v1.5.0"},

		{"candidate for a patch", []string{"v1.2.0"}, "patch", true, "v1.2.1-rc.1"},
		{"candidate for a minor", []string{"v1.2.0"}, "minor", true, "v1.3.0-rc.1"},
		{"candidate continues its line", []string{"v1.2.0", "v1.3.0-rc.1"}, "patch", true, "v1.3.0-rc.2"},
		{"candidate counts to the highest", []string{"v1.3.0-rc.1", "v1.3.0-rc.2"}, "patch", true, "v1.3.0-rc.3"},

		// Promotion ignores the bump, or the candidate is stranded at a version
		// that never ships.
		{"promote a candidate", []string{"v1.2.0", "v1.3.0-rc.1"}, "patch", false, "v1.3.0"},
		{"promote, bump says minor", []string{"v1.2.0", "v1.3.0-rc.1"}, "minor", false, "v1.3.0"},
		{"promote, bump says major", []string{"v1.2.0", "v1.3.0-rc.1"}, "major", false, "v1.3.0"},
		{"promote the last candidate", []string{"v1.3.0-rc.1", "v1.3.0-rc.2"}, "patch", false, "v1.3.0"},

		// Tags arrive in whatever order git prints them, which is lexical.
		{"semver order, not lexical", []string{"v1.10.0", "v1.9.0"}, "patch", false, "v1.10.1"},
		{"rc sorts before its release", []string{"v1.3.0", "v1.3.0-rc.1"}, "patch", false, "v1.3.1"},
		{"unsorted input", []string{"v2.0.0", "v1.0.0", "v1.5.0"}, "patch", false, "v2.0.1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			latest, have, parsed := latestOf(c.tags)
			got, err := compute(latest, have, parsed, c.bump, c.candidate)
			if err != nil {
				t.Fatalf("compute: %v", err)
			}
			if got.String() != c.want {
				t.Errorf("tags %v bump %q candidate %v: got %s, want %s",
					c.tags, c.bump, c.candidate, got, c.want)
			}
		})
	}
}

func TestComputeRejectsUnknownBump(t *testing.T) {
	if _, err := compute(version{}, true, nil, "sideways", false); err == nil {
		t.Fatal("expected an error for an unknown bump")
	}
}

func TestParse(t *testing.T) {
	good := map[string]version{
		"v1.2.3":         {major: 1, minor: 2, patch: 3},
		"1.2.3":          {major: 1, minor: 2, patch: 3},
		" v1.2.3 ":       {major: 1, minor: 2, patch: 3},
		"v1.2.3-rc.4":    {major: 1, minor: 2, patch: 3, rc: 4},
		"v0.0.0":         {},
		"v10.20.30":      {major: 10, minor: 20, patch: 30},
		"v1.2.3-rc.1000": {major: 1, minor: 2, patch: 3, rc: 1000},
	}
	for tag, want := range good {
		got, ok := parse(tag)
		if !ok {
			t.Errorf("parse(%q): rejected, want %v", tag, want)
			continue
		}
		if got != want {
			t.Errorf("parse(%q) = %v, want %v", tag, got, want)
		}
	}

	// A tag that does not parse must be dropped rather than guessed at: one
	// misread tag picks the wrong version for every release after it.
	bad := []string{
		"", "v", "nightly", "v1.2", "v1.2.3.4", "v1.2.x",
		"v1.2.3-rc.0", "v1.2.3-rc.x", "v1.2.3-rc.", "v1.2.3-beta.1",
		"v-1.2.3", "v1.-2.3",
	}
	for _, tag := range bad {
		if got, ok := parse(tag); ok {
			t.Errorf("parse(%q) = %v, want rejected", tag, got)
		}
	}
}

func TestLessOrdersCandidateBeforeItsRelease(t *testing.T) {
	rc, _ := parse("v1.3.0-rc.1")
	final, _ := parse("v1.3.0")
	if !rc.less(final) {
		t.Error("v1.3.0-rc.1 must sort before v1.3.0")
	}
	if final.less(rc) {
		t.Error("v1.3.0 must not sort before v1.3.0-rc.1")
	}
}
