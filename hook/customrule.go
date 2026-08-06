package hook

import (
	"context"
	"fmt"
	"sort"
)

// RegisterCustomRules adds every enabled plugin entry, returning one error per
// unusable entry. Errors rather than panics, unlike a built-in: a bad built-in
// is our bug, a bad config entry is a typo that must not take the hook down.
func RegisterCustomRules() []error {
	entries := LoadCustomRules()
	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids) // registration order decides SingleFailure precedence

	var errs []error
	for _, id := range ids {
		if !entries[id].Enabled {
			continue
		}
		r, err := customRule(id, entries[id])
		if err != nil {
			errs = append(errs, fmt.Errorf("custom_rules.%s: %w", id, err))
			continue
		}
		Register(r)
	}
	return errs
}

// customRule builds the registry entry for one plugin role.
func customRule(id string, c CustomRule) (Rule, error) {
	t, err := transportFor(c)
	if err != nil {
		return Rule{}, err
	}
	if len(c.Events) == 0 {
		return Rule{}, fmt.Errorf("needs at least one event")
	}

	// Rules sharing an endpoint share one call. Keying on the endpoint is what
	// turns five roles from one binary into one spawn instead of five.
	key := c.ServerURL + c.StartupCmd
	broker.join(key, id)
	ask := func(ctx context.Context, ev *Event) *PluginResponse {
		return broker.response(ctx, key, t, ev, id)
	}

	r := Rule{
		ID:               id,
		Description:      "custom rule (" + string(c.Verb) + ")",
		Events:           c.Events,
		Tools:            c.Tools,
		Timeout:          c.Timeout,
		EnabledByDefault: true, // reaching here means the entry already said so
	}

	// The declared verb picks both the reply field read and the slot the rule
	// occupies. A plugin returning a decision while declared as an advisor is
	// ignored, not obeyed.
	switch c.Verb {
	case VerbCheck:
		r.Check = func(ctx context.Context, ev *Event) []Finding {
			if resp := ask(ctx, ev); resp != nil {
				return resp.Findings
			}
			return nil
		}
	case VerbAdvise:
		r.Advise = func(ctx context.Context, ev *Event) *Advice {
			if resp := ask(ctx, ev); resp != nil && resp.Advice != "" {
				return &Advice{Text: resp.Advice}
			}
			return nil
		}
	case VerbGate:
		r.Gate = func(ctx context.Context, ev *Event) *Decision {
			if resp := ask(ctx, ev); resp != nil {
				return resp.Decision
			}
			return nil
		}
	case VerbRewrite:
		r.Rewrite = func(ctx context.Context, ev *Event) *Mutation {
			if resp := ask(ctx, ev); resp != nil {
				return resp.Mutation
			}
			return nil
		}
	case VerbDisplay:
		r.Display = func(ctx context.Context, ev *Event) *DisplayContent {
			if resp := ask(ctx, ev); resp != nil && resp.Display != "" {
				return &DisplayContent{Text: resp.Display}
			}
			return nil
		}
	default:
		return Rule{}, fmt.Errorf(
			"verb must be one of Check, Advise, Gate, Rewrite, Display (got %q)", c.Verb)
	}
	return r, nil
}
