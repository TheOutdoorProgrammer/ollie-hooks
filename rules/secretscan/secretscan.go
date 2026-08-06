package secretscan

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// scanTimeout bounds the scan under whatever the framework already allows:
// betterleaks handles a prompt in single-digit milliseconds, so anything near
// this means something is badly wrong.
const scanTimeout = 5 * time.Second

// leak is one betterleaks finding. Match and Secret come back redacted, so
// nothing here carries the credential itself.
type leak struct {
	RuleID      string `json:"RuleID"`
	Description string `json:"Description"`
	StartLine   int    `json:"StartLine"`
}

// checkSecrets blocks a prompt carrying something credential-shaped. This is
// the only event that can stop text before it reaches the API: PreToolUse is
// too late, and a commit-time gate never sees a pasted credential at all.
func checkSecrets(ctx context.Context, ev *hook.Event) []hook.Finding {
	cfg := loadConfig()
	if strings.TrimSpace(ev.Prompt) == "" {
		return nil
	}
	// RequiresBinaries is static, so it vets betterleaks — not whatever the user
	// pointed `binary` at. That substitution has to fail closed here instead,
	// and the hint must name the tool they actually chose.
	if cfg.Binary != defaultConfig().Binary && !hook.Installed(cfg.Binary) {
		return []hook.Finding{hook.MissingBinary(RuleID, cfg.Binary,
			"install "+cfg.Binary+", or unset [rules."+RuleID+"].binary to use "+
				defaultConfig().Binary)}
	}

	ctx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()
	res, ok := hook.Exec(ctx, ev.Prompt, cfg.Binary,
		"stdin", "--no-banner", "--redact="+strconv.Itoa(cfg.Redact),
		"--report-format", "json", "--report-path", "-")
	if !ok {
		return unscanned(cfg.Binary, "could not be run")
	}
	// betterleaks exits 0 clean and 1 when it finds something. Any other code is
	// the scanner failing, which must not read the same as a clean prompt.
	if res.ExitCode != 0 && res.ExitCode != 1 {
		return unscanned(cfg.Binary, "exited "+strconv.Itoa(res.ExitCode))
	}

	leaks, readable := parseLeaks(res.Stdout)
	if !readable {
		return unscanned(cfg.Binary, "produced a report that could not be read")
	}
	if len(leaks) == 0 {
		return nil
	}
	return []hook.Finding{{Message: describe(leaks)}}
}

// unscanned is the fail-closed report. A prompt nobody scanned has to look
// different from a clean one, or every failure mode of the scanner silently
// becomes a pass.
func unscanned(binary, what string) []hook.Finding {
	return []hook.Finding{{Message: `
		` + binary + ` ` + what + `, so this prompt was NOT scanned for
		credentials. Fix the tool or disable the secret-scan rule — failing
		quietly here would defeat the point of the check.`}}
}

// parseLeaks reads the scanner's JSON report. readable is false when the output
// is not a report at all, which is a failed scan rather than a clean one: this
// rule exists to catch secrets, so unreadable must never read as "none found".
func parseLeaks(out string) (leaks []leak, readable bool) {
	out = strings.TrimSpace(out)
	if out == "" || !strings.HasPrefix(out, "[") {
		return nil, false
	}
	if err := json.Unmarshal([]byte(out), &leaks); err != nil {
		return nil, false
	}
	return leaks, true
}

// describe names what was found and where, never the value. Repeating the
// secret into the block reason would put it straight back in the transcript.
func describe(leaks []leak) string {
	kinds := make([]string, 0, len(leaks))
	seen := map[string]bool{}
	for _, l := range leaks {
		name := l.RuleID
		if name == "" {
			name = "unrecognised credential"
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		kinds = append(kinds, name+" (line "+strconv.Itoa(l.StartLine)+")")
	}
	return `
		Blocked: this prompt contains what looks like a live credential —
		` + strings.Join(kinds, ", ") + `. It was NOT sent. The value is
		deliberately not repeated here, since that would put it back in the
		transcript. Remove or replace the credential and resend. If this is a
		false positive, allowlist the pattern in your betterleaks config rather
		than disabling the rule.`
}
