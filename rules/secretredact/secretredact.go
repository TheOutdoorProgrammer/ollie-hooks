package secretredact

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/internal/betterleaks"
)

// textFields are the tool-output keys worth scanning. The rest of the object
// (interrupted, isImage) is preserved untouched.
var textFields = []string{"stdout", "stderr", "output", "content"}

// redactToolOutput replaces credentials in a tool's result before Claude reads
// it. Once a secret reaches the model's context it is in the transcript for
// good, and nothing downstream can take it back.
func redactToolOutput(ctx context.Context, ev *hook.Event) *hook.Mutation {
	cfg := loadConfig()
	if !slices.Contains(cfg.Tools, ev.ToolName) || len(ev.ToolResponse) == 0 {
		return nil
	}
	var resp map[string]any
	if err := json.Unmarshal(ev.ToolResponse, &resp); err != nil {
		return nil // not an object we understand — leave it alone
	}

	var scanned strings.Builder
	for _, k := range textFields {
		if s, ok := resp[k].(string); ok && s != "" {
			scanned.WriteString(s)
			scanned.WriteByte('\n')
		}
	}
	if scanned.Len() == 0 {
		return nil
	}

	leaks, err := betterleaks.Scan(ctx, cfg.Binary, scanned.String(), 0)
	if err != nil {
		return nil
	}
	found := secretsByValue(leaks)
	if len(found) == 0 {
		return nil
	}

	kinds := map[string]bool{}
	changed := false
	for _, k := range textFields {
		s, ok := resp[k].(string)
		if !ok || s == "" {
			continue
		}
		out, hit := replaceAll(s, found, kinds)
		if hit {
			resp[k] = out
			changed = true
		}
	}
	if !changed {
		return nil
	}

	// Decoded and re-emitted whole: updatedToolOutput replaces the entire
	// object, so dropping interrupted or isImage would silently change the
	// result in ways the rule never intended.
	return &hook.Mutation{UpdatedOutput: resp, Note: note(kinds)}
}

// replaceAll swaps every known secret for a placeholder naming its kind, so the
// model can see that a value was removed rather than that one never existed.
func replaceAll(s string, found map[string]string, kinds map[string]bool) (string, bool) {
	hit := false
	for secret, kind := range found {
		if secret == "" || !strings.Contains(s, secret) {
			continue
		}
		s = strings.ReplaceAll(s, secret, "<redacted:"+kind+">")
		kinds[kind] = true
		hit = true
	}
	return s, hit
}

// secretsByValue maps each literal secret to its credential kind, so replaceAll
// can swap the value for a placeholder naming that kind. The scan runs at
// redact=0 because the value itself is what we match and replace on.
func secretsByValue(leaks []betterleaks.Leak) map[string]string {
	found := make(map[string]string, len(leaks))
	for _, l := range leaks {
		if l.Secret == "" {
			continue
		}
		kind := l.RuleID
		if kind == "" {
			kind = "secret"
		}
		found[l.Secret] = kind
	}
	return found
}

// note tells the model a value was removed and why, so it does not treat the
// placeholder as the real output or go hunting for the original.
func note(kinds map[string]bool) string {
	names := make([]string, 0, len(kinds))
	for k := range kinds {
		names = append(names, k)
	}
	sort.Strings(names)
	return `This tool's output contained credentials (` + strings.Join(names, ", ") +
		`), which were replaced with <redacted:kind> placeholders before you saw
		it. The real values are deliberately unavailable — do not try to recover
		them, and do not treat a placeholder as a usable value.`
}
