// Package hook is the ollie-hooks rule API: everything a rule author touches.
// A rule declares the event and tools it applies to and exactly one behaviour
// (Check, Rewrite, Gate, Display or Advise); the registry runs the matching ones
// and Decide turns their output into the JSON envelope Claude Code expects.
//
// Rules fail open by design — a rule that errors or overruns its timeout is
// abandoned, never allowed to wedge a session.
package hook

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	toon "github.com/toon-format/toon-go"
)

// Decide runs the rules for an event and returns the JSON response to print
// (empty string = allow with no output). Precedence is deliberate and is the
// one place it lives, so no future rule can invert it:
//
//  1. Findings win. If any Check rule produces findings, the event is
//     denied/advised — full stop. (SingleFailure only changes WHICH findings
//     Run returns, never whether findings exist, so a SingleFailure deny still
//     lands here and short-circuits the rewrite below.)
//  2. Only when nothing denied does a Rewrite rule transform the call — the
//     input on PreToolUse, the result on PostToolUse. A blocked call is
//     therefore never silently rewritten.
func Decide(ev *Event) (string, error) {
	// One event per Decide, so last event.s plugin answers must not be reused.
	broker.reset()
	// MessageDisplay replaces streamed text and can express nothing else. It is
	// also the hottest event — once per delta — so it takes the short path and
	// never pays for the rules it could not use anyway.
	if ev.HookEventName == MessageDisplay {
		if dc := RunDisplays(ev); dc != nil {
			return respondDisplay(ev.HookEventName, dc)
		}
		return "", nil
	}
	checks := RunChecks(ev)
	findings := checks.Findings
	// A Display rule on these events only echoes text at the user; it never
	// blocks, so it rides alongside findings instead of competing with them.
	var echo string
	if dc := RunDisplays(ev); dc != nil {
		echo = dc.Text
	}
	// Advice is not a violation, so it rides alongside a block rather than
	// competing with it: the model reads both the reason and the context.
	// Advisory-severity checks join it — same text, downgraded by the user.
	advice := joinAdvice(append(checks.Advisories, RunAdvice(ev)...))
	if len(findings) > 0 {
		return respond(ev.HookEventName, findings, echo, advice)
	}
	// Gate before Rewrite: a gate can approve or deny outright, and running it
	// only after Run means a Check deny short-circuits before an interactive
	// gate ever opens.
	if d := RunGates(ev); d != nil {
		return respondDecision(ev.HookEventName, d, echo, advice)
	}
	// Rewrites are event-scoped by the registry: PreToolUse transforms the
	// input, PostToolUse the result Claude is about to read.
	if m := RunRewrites(ev); m != nil {
		return respondRewrite(ev.HookEventName, m, echo, advice)
	}
	if echo != "" || advice != "" {
		return respondPassive(ev.HookEventName, echo, advice)
	}
	return "", nil
}

// respond builds the event-appropriate block response. The envelope must be
// JSON — Claude Code parses hook stdout as JSON and ignores anything else —
// but the reason payload the model reads is the findings list encoded as
// TOON (always a list, even for a single finding) for token economy.
func respond(event EventName, findings []Finding, echo, advice string) (string, error) {
	encode := func(fs []Finding) (string, error) {
		return toon.MarshalString(map[string][]Finding{"findings": fs})
	}
	findings = expandFindings(findings, wrapWidth())
	_, reason, err := fitFindings(findings, encode)
	if err != nil {
		// TOON is an optimization, not a requirement — fall back to JSON
		// findings rather than dropping the block.
		raw, jerr := json.Marshal(map[string][]Finding{"findings": findings})
		if jerr != nil {
			return "", fmt.Errorf("encoding findings: %w", err)
		}
		reason = string(raw)
	}

	var envelope any
	switch event {
	case PreToolUse:
		envelope = map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "deny",
				"permissionDecisionReason": reason,
			},
		}
	case PostToolUse, PostToolUseFailure:
		// The tool already ran; "block" can't undo it but feeds the findings
		// back so the model corrects course.
		envelope = map[string]any{
			"decision": "block",
			"reason":   reason,
		}
	case UserPromptSubmit:
		// suppressOriginalPrompt keeps the blocked text out of the message shown
		// back to the user: without it, refusing to send a secret reprints it.
		envelope = map[string]any{
			"decision":               "block",
			"reason":                 reason,
			"suppressOriginalPrompt": true,
		}
	case Stop, SubagentStop, PostToolBatch, ConfigChange, PreCompact,
		UserPromptExpansion:
		// These block through the top-level decision. Stop's block asks Claude
		// to keep going rather than undoing anything.
		envelope = map[string]any{
			"decision": "block",
			"reason":   reason,
		}
	default:
		return "", fmt.Errorf("no envelope for event %q", event)
	}

	// systemMessage is for the user's eyes and is independent of the findings
	// the model reads, so an echoing Display rule survives alongside a block.
	if m, ok := envelope.(map[string]any); ok {
		if echo != "" {
			m["systemMessage"] = systemMessageText(echo)
		}
		if advice != "" {
			attachAdvice(m, event, advice)
		}
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("encoding envelope: %w", err)
	}
	return string(raw), nil
}

// respondPassive emits output that neither blocks nor decides: an echo for the
// user, context for the model, or both.
func respondPassive(event EventName, echo, advice string) (string, error) {
	envelope := map[string]any{}
	if echo != "" {
		envelope["systemMessage"] = systemMessageText(echo)
	}
	if advice != "" {
		attachAdvice(envelope, event, advice)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("encoding passive envelope: %w", err)
	}
	return string(raw), nil
}

// attachAdvice adds additionalContext under the event's hookSpecificOutput,
// merging with whatever the envelope already carries there.
func attachAdvice(envelope map[string]any, event EventName, advice string) {
	hso, ok := envelope["hookSpecificOutput"].(map[string]any)
	if !ok {
		hso = map[string]any{"hookEventName": string(event)}
		envelope["hookSpecificOutput"] = hso
	}
	hso["additionalContext"] = advice
}

// joinAdvice concatenates every rule's advice, labelled so the model can tell
// which check produced which statement.
func joinAdvice(advice []Advice) string {
	if len(advice) == 0 {
		return ""
	}
	parts := make([]string, 0, len(advice))
	for _, a := range advice {
		parts = append(parts, "["+a.Rule+"] "+NormalizeProse(a.Text))
	}
	return strings.Join(parts, "\n\n")
}

// respondRewrite builds the envelope that transforms a tool call. It carries no
// permissionDecision — the rewrite is transparent, leaving the normal
// permission flow to act on the new value.
func respondRewrite(event EventName, m *Mutation, echo, advice string) (string, error) {
	hso := map[string]any{"hookEventName": string(event)}
	switch {
	case m.UpdatedInput != nil:
		hso["updatedInput"] = m.UpdatedInput
	case m.UpdatedOutput != nil:
		hso["updatedToolOutput"] = m.UpdatedOutput
	}
	if note := NormalizeProse(m.Note); note != "" {
		advice = strings.TrimSpace(advice + "\n\n[" + m.Rule + "] " + note)
	}
	if advice != "" {
		hso["additionalContext"] = advice
	}
	envelope := map[string]any{"hookSpecificOutput": hso}
	if echo != "" {
		envelope["systemMessage"] = systemMessageText(echo)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("encoding rewrite envelope: %w", err)
	}
	return string(raw), nil
}

// systemMessageText guards one platform rule: Claude Code silently DROPS a
// systemMessage starting with whitespace. Rules supply their own header line,
// which also keeps "<Event> says: " from misaligning the output beneath it.
func systemMessageText(s string) string {
	if s != "" && (s[0] == '\n' || s[0] == ' ' || s[0] == '\t') {
		return "↓" + s
	}
	return s
}

// respondDisplay shows a Display rule's text to the user. The field differs by
// event: MessageDisplay replaces the streamed text (needs verbose off), the
// rest emit a separate systemMessage, which verbose never gates.
func respondDisplay(event EventName, dc *DisplayContent) (string, error) {
	var envelope any
	switch event {
	case MessageDisplay:
		envelope = map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":  "MessageDisplay",
				"displayContent": dc.Text,
			},
		}
	case Stop, PostToolUse, PostToolUseFailure, Notification:
		envelope = map[string]any{"systemMessage": systemMessageText(dc.Text)}
	default:
		return "", fmt.Errorf("no display envelope for event %q", event)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("encoding display envelope: %w", err)
	}
	return string(raw), nil
}

// respondDecision builds the envelope for a Gate verdict, in the shape that
// event actually reads: three events, three different shapes. echo and advice
// ride along, or a gate that merely allows a call would swallow every other
// rule.s output.
func respondDecision(event EventName, d *Decision, echo, advice string) (string, error) {
	hso := map[string]any{"hookEventName": string(event)}
	// additionalContext is documented for neither of the two events below, so
	// it only rides along where it is actually read.
	carriesAdvice := false

	switch event {
	case PreToolUse:
		if err := allowed(event, d.Permission,
			PermissionAllow, PermissionDeny, PermissionAsk, PermissionDefer); err != nil {
			return "", err
		}
		hso["permissionDecision"] = d.Permission
		hso["permissionDecisionReason"] = d.Reason
		if d.UpdatedInput != nil {
			hso["updatedInput"] = d.UpdatedInput
		}
		carriesAdvice = true

	case PermissionRequest:
		// behavior, not permissionDecision, and updatedInput nests one level
		// deeper than it does on PreToolUse.
		if err := allowed(event, d.Permission, PermissionAllow, PermissionDeny); err != nil {
			return "", err
		}
		decision := map[string]any{"behavior": d.Permission}
		if d.Permission == PermissionAllow && d.UpdatedInput != nil {
			decision["updatedInput"] = d.UpdatedInput
		}
		if d.Permission == PermissionDeny && d.Reason != "" {
			decision["message"] = d.Reason // not permissionDecisionReason
		}
		hso["decision"] = decision

	case Elicitation, ElicitationResult:
		if err := allowed(event, d.Permission,
			ActionAccept, ActionDecline, ActionCancel); err != nil {
			return "", err
		}
		hso["action"] = d.Permission
		if d.Permission == ActionAccept && d.Content != nil {
			hso["content"] = d.Content
		}

	default:
		return "", fmt.Errorf("no decision envelope for event %q", event)
	}

	if carriesAdvice && advice != "" {
		hso["additionalContext"] = advice
	}
	envelope := map[string]any{"hookSpecificOutput": hso}
	// systemMessage is universal, so the user still sees an echo either way.
	if echo != "" {
		envelope["systemMessage"] = systemMessageText(echo)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("encoding decision envelope: %w", err)
	}
	return string(raw), nil
}

// allowed rejects a verdict the event cannot express. Claude Code would drop
// the field, and on PermissionRequest that reads as a denial rather than as
// nothing having happened.
func allowed(event EventName, got string, want ...string) error {
	if slices.Contains(want, got) {
		return nil
	}
	return fmt.Errorf("%s cannot express permission %q; use one of %s",
		event, got, strings.Join(want, ", "))
}
