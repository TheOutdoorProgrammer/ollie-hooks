// Package betterleaks centralizes how the secret rules invoke the betterleaks
// scanner and read its report, so the exit-code and JSON contract lives in one
// place rather than drifting between two rules.
package betterleaks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// Leak is one betterleaks finding. Secret is populated only when the scan runs
// with redact=0; otherwise it comes back redacted along with the match.
type Leak struct {
	RuleID      string `json:"RuleID"`
	Description string `json:"Description"`
	StartLine   int    `json:"StartLine"`
	Secret      string `json:"Secret"`
}

// Scan runs binary over input at the given redaction percent and returns the
// leaks. A non-nil error means the input was NOT scanned (tool absent, bad
// exit, unreadable report) and must never read as clean; its text is a reason.
func Scan(ctx context.Context, binary, input string, redact int) ([]Leak, error) {
	res, ok := hook.Exec(ctx, input, binary,
		"stdin", "--no-banner", "--redact="+strconv.Itoa(redact),
		"--report-format", "json", "--report-path", "-")
	if !ok {
		return nil, errors.New("could not be run")
	}
	// betterleaks exits 0 clean and 1 when it finds something. Any other code is
	// the scanner failing, which must not read the same as a clean input.
	if res.ExitCode != 0 && res.ExitCode != 1 {
		return nil, fmt.Errorf("exited %d", res.ExitCode)
	}
	leaks, readable := parseReport(res.Stdout)
	if !readable {
		return nil, errors.New("produced a report that could not be read")
	}
	return leaks, nil
}

// parseReport reads a betterleaks JSON report. readable is false for output
// that is not a report: a failed scan, not a clean one, so junk can never
// read as "no leaks found".
func parseReport(out string) (leaks []Leak, readable bool) {
	out = strings.TrimSpace(out)
	if out == "" || !strings.HasPrefix(out, "[") {
		return nil, false
	}
	if err := json.Unmarshal([]byte(out), &leaks); err != nil {
		return nil, false
	}
	return leaks, true
}
