package hook

import "strings"

// Severity is how forcefully a rule's findings are delivered. The rule decides
// WHAT it found; the user decides what that is worth — advisory on a shared
// machine, hard-block in CI. Not the rule author's call.
type Severity string

const (
	// SeverityBlock denies the call and feeds the findings back as the reason.
	SeverityBlock Severity = "block"
	// SeverityAdvisory delivers the same findings as context, without blocking —
	// the channel the Advise verb uses.
	SeverityAdvisory Severity = "advisory"
	// SeverityOff skips the rule, exactly like disabling it.
	SeverityOff Severity = "off"
)

// RuleSeverity resolves [rules.<id>].severity, falling back to the rule's own
// default. An unrecognised value falls back too, rather than guessing: silently
// treating a typo as "off" would disable a check the user believes is running.
func RuleSeverity(id string, byDefault Severity) Severity {
	cfg := struct {
		Severity string `toml:"severity"`
	}{}
	if !loadConfig(id, &cfg) {
		return byDefault
	}
	switch Severity(strings.ToLower(strings.TrimSpace(cfg.Severity))) {
	case SeverityBlock:
		return SeverityBlock
	case SeverityAdvisory:
		return SeverityAdvisory
	case SeverityOff:
		return SeverityOff
	}
	return byDefault
}

// severity is the rule's effective severity, defaulting to block: a Check rule
// that declares nothing is asserting a violation.
func (r Rule) severity() Severity {
	def := r.DefaultSeverity
	if def == "" {
		def = SeverityBlock
	}
	return RuleSeverity(r.ID, def)
}
