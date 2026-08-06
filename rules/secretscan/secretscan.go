package secretscan

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/internal/betterleaks"
)

// scanTimeout bounds the scan under whatever the framework already allows:
// betterleaks handles a prompt in single-digit milliseconds, so anything near
// this means something is badly wrong.
const scanTimeout = 5 * time.Second

// checkSecrets blocks a prompt carrying something credential-shaped. This is
// the only event that can stop text before it reaches the API: PreToolUse is
// too late, and a commit-time gate never sees a pasted credential at all.
func checkSecrets(ctx context.Context, ev *hook.Event) []hook.Finding {
	cfg := loadConfig()
	if strings.TrimSpace(ev.Prompt) == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()
	leaks, err := betterleaks.Scan(ctx, cfg.Binary, ev.Prompt, cfg.Redact)
	if err != nil {
		return unscanned(cfg.Binary, err.Error())
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

// describe names what was found and where, never the value. Repeating the
// secret into the block reason would put it straight back in the transcript.
func describe(leaks []betterleaks.Leak) string {
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
