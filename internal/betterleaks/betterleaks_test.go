package betterleaks

import "testing"

// Output that is not a report means the scan failed, NOT that the input was
// clean. Treating junk as "no findings" turns every crash into a silent pass.
func TestParseReportRejectsJunk(t *testing.T) {
	for _, in := range []string{"", "   ", "not json", "{}", "INF scanned 12 bytes"} {
		if _, readable := parseReport(in); readable {
			t.Errorf("parseReport(%q) reported a readable report", in)
		}
	}
}

func TestParseReportAcceptsAnEmptyReport(t *testing.T) {
	leaks, readable := parseReport("[]")
	if !readable {
		t.Fatal("[] is a valid report meaning nothing was found")
	}
	if len(leaks) != 0 {
		t.Errorf("got %v, want no leaks", leaks)
	}
}
