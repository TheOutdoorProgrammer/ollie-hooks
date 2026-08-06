package hook

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

// defaultRuleTimeout bounds a rule that doesn't declare its own Timeout. It
// matches the value the ollie-hooks settings.json entry used to carry, so
// moving enforcement in-process is behavior-preserving for the fast rules.
const defaultRuleTimeout = 15

// Rule is one registered check over a hook event: an ID, the event it runs on,
// the tools it applies to, and a pure check. Rules live one-per-file in
// rules/<name>/ and register explicitly via their package Register.
type Rule struct {
	ID          string
	Description string
	// Events this rule runs on; see eventCaps for what each can express. More
	// than one when the same behaviour belongs to several — a Bash echo needs
	// both PostToolUse and PostToolUseFailure, and splitting that into two
	// rules would give the user two things to configure for one feature.
	Events []EventName
	Tools  []string // tool names the rule applies to; empty = every tool
	// SingleFailure marks a precise-diagnosis rule: when it fires, its
	// findings are returned ALONE — no other rule's output piles on — so the
	// agent gets one clear instruction. Registration order decides which
	// diagnosis wins when several could fire; register the most specific
	// first (init.go).
	SingleFailure bool
	// Timeout is this rule's wall-clock budget in seconds (0 = the default).
	// Enforced in-process, and overridable per rule in config: the settings
	// entry is deliberately long so this is the deadline that actually fires.
	Timeout int
	Check   func(ctx context.Context, ev *Event) []Finding
	// Rewrite, if set, transforms the tool input before it runs (PreToolUse
	// updatedInput) instead of producing findings. Applied only when no Check
	// rule denied, so a blocked command is never silently rewritten.
	Rewrite func(ctx context.Context, ev *Event) *Mutation
	// Gate returns an explicit allow/deny Decision (nil = defer). Unlike Check it
	// can APPROVE a call — what the review gate needs. Runs only after no Check
	// denied; its ctx is cancelled at Timeout (wire it into any child process).
	Gate func(ctx context.Context, ev *Event) *Decision
	// Display rewrites the assistant text shown on screen (MessageDisplay
	// displayContent). Display-only: the transcript and the model's own context
	// keep the original, so this can never destroy what was actually said.
	Display func(ctx context.Context, ev *Event) *DisplayContent
	// Advise hands the model context without blocking (additionalContext).
	// Unlike Check it is not a violation, so it renders as context rather than
	// a hook error; unlike Display it reaches Claude rather than the user.
	Advise func(ctx context.Context, ev *Event) *Advice
	// RequiresBinaries are external programs the rule cannot work without. These
	// are executables on PATH, not Claude Code tools — an enabled rule missing one
	// reports that instead of running, rather than passing silently.
	RequiresBinaries []Binary
	// Binaries reports the same thing for tools only known once config is read,
	// such as a scanner the user repointed. Without it those go unvetted, and a
	// rule whose tool is missing looks exactly like one that found nothing.
	Binaries func() []Binary
	// EnabledByDefault rules run without configuration. Anything that blocks,
	// rewrites or costs real time should leave this false: a tool that starts
	// denying calls the moment it is installed is a bad first impression.
	EnabledByDefault bool
	// DefaultSeverity is how this rule's findings arrive unless the user says
	// otherwise (empty = block). Only Check rules use it.
	DefaultSeverity Severity
	// FailClosedOnTimeout blocks instead of skipping when a Check or Gate rule
	// overruns. Off by default — a hung rule must never wedge the session — so
	// set it only where NOT running is the failure, as secret-scan does.
	FailClosedOnTimeout bool
	// Doc is the rule's paragraph in the generated reference; Description stays
	// the one-liner.
	Doc string
	// Config is the rule's defaults, the same value its loader layers the user's
	// section over, so documented defaults cannot drift from real ones. Every
	// exported field needs a toml and a doc tag or Register rejects the rule.
	Config any
	// DocTable adds a generated reference table to this rule's docs, for a set
	// of legal values the config type alone cannot spell out.
	DocTable func() DocTable
}

// Advice is non-blocking context for the model. Phrase it as statements of
// fact: text shaped like an out-of-band instruction trips Claude's
// prompt-injection defences and gets shown to the user instead.
type Advice struct {
	// Rule is filled in by the framework with the firing rule's ID; any value
	// you set here is overwritten.
	Rule string
	Text string
}

// A rule sets EXACTLY ONE of Check, Rewrite, Gate, Display or Advise.

// DisplayContent replaces the on-screen text for one MessageDisplay delta.
// Text is emitted verbatim, so a rule that changes nothing must return the
// delta unchanged rather than nil-ing out the screen.
type DisplayContent struct {
	// Rule is filled in by the framework with the firing rule's ID; any value
	// you set here is overwritten.
	Rule string
	Text string
}

// Verdict values for Decision.Permission. Which set applies depends on the
// event, and respondDecision rejects one from the wrong set rather than
// emitting an envelope Claude Code cannot read.
const (
	// PreToolUse and PermissionRequest. Ask and Defer are PreToolUse only.
	PermissionAllow = "allow"
	PermissionDeny  = "deny"
	PermissionAsk   = "ask"
	PermissionDefer = "defer"
	ActionAccept    = "accept"
	ActionDecline   = "decline"
	ActionCancel    = "cancel"
)

// Decision is a Gate's verdict. nil = defer to the normal flow, though on
// PermissionRequest that is not neutral: a session with no prompt to show
// denies the call when nothing decides.
type Decision struct {
	// Rule is filled in by the framework with the firing rule's ID; any value
	// you set here is overwritten.
	Rule string
	// Permission is the verdict, from the constants above. PreToolUse and
	// PermissionRequest take allow/deny; Elicitation takes accept/decline/cancel.
	Permission string
	// Reason explains the verdict. Multiline is fine, it lands in a JSON string
	// rather than the rendered findings table.
	Reason string
	// UpdatedInput replaces the tool's arguments. Allowed calls only, and it
	// replaces the WHOLE object, so start from the decoded input.
	UpdatedInput map[string]any
	// Content is the form the user would have filled in, for an accepted
	// Elicitation.
	Content map[string]any
}

// Mutation transforms a tool call instead of blocking it. Which field applies
// depends on the event; setting the wrong one is ignored and traced.
type Mutation struct {
	// Rule is filled in by the framework with the firing rule's ID; any value
	// you set here is overwritten.
	Rule string
	// UpdatedInput replaces a tool's arguments before it runs (PreToolUse). It
	// replaces the WHOLE object, so start from the decoded input or fields the
	// rule never touched are dropped.
	UpdatedInput map[string]any
	// UpdatedOutput replaces a tool's result before Claude sees it
	// (PostToolUse). It must match that tool's output shape — Bash is
	// {stdout, stderr, interrupted, isImage} — or it is discarded silently.
	UpdatedOutput any
	// Note rides along as additionalContext. A rewrite the model cannot see is
	// confusing: it should learn that its input or output changed, and why.
	Note string
}

// Finding is one violation: which rule fired and what the model should do
// about it — never a bare "failed". The findings list is rendered as a table
// into the block reason the model reads.
type Finding struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

var registry []Rule

// SwapRegistry installs rules and returns a func that restores the previous
// set. It is the seam hooktest.Sandbox uses to isolate a test's rules without
// reaching the unexported registry.
func SwapRegistry(rules []Rule) (restore func()) {
	saved := registry
	registry = rules
	return func() { registry = saved }
}

// registered reports whether a rule id is already taken, for callers that must
// not risk Register's panic.
func registered(id string) bool {
	for _, r := range registry {
		if r.ID == id {
			return true
		}
	}
	return false
}

// Register adds a rule; an empty ID/Event, no behavior, or duplicate ID panics
// at startup rather than producing confusing double findings.
func Register(r Rule) {
	if r.ID == "" || len(r.Events) == 0 || r.behaviours() != 1 {
		panic("hooks: Register needs a non-empty ID/Events and exactly one of " +
			"Check, Rewrite, Gate, Display or Advise")
	}
	if registered(r.ID) {
		panic("hooks: rule " + r.ID + " registered twice")
	}
	if err := checkConfigTags(r.ID, r.Config); err != nil {
		panic("hooks: rule " + r.ID + ": " + err.Error())
	}
	// Fail loudly at startup, not silently forever: an event whose envelope has
	// no path for this verb would accept the rule and never run it.
	v := r.verb()
	if r.FailClosedOnTimeout && v != VerbCheck && v != VerbGate {
		panic("hooks: rule " + r.ID + " sets FailClosedOnTimeout on a " + string(v) +
			" rule; only Check and Gate can block on timeout")
	}
	for _, ev := range r.Events {
		if !knownEvents[ev] {
			panic("hooks: rule " + r.ID + " registers on unknown event " + string(ev))
		}
		if !eventActionable(ev) {
			panic("hooks: rule " + r.ID + " registers on " + string(ev) +
				", which delivers no actionable hook output")
		}
		if !eventAllows(ev, v) {
			panic("hooks: rule " + r.ID + " uses " + string(v) + " on " + string(ev) +
				", which cannot express it")
		}
	}
	registry = append(registry, r)
}

// handles reports whether this rule runs on an event.
func (r Rule) handles(event EventName) bool {
	return slices.Contains(r.Events, event)
}

// behaviours counts the verbs a rule sets. Registration requires exactly one:
// two would make the rule's envelope ambiguous.
func (r Rule) behaviours() int {
	n := 0
	for _, set := range []bool{
		r.Check != nil, r.Rewrite != nil, r.Gate != nil, r.Display != nil, r.Advise != nil,
	} {
		if set {
			n++
		}
	}
	return n
}

// verb reports which behaviour this rule sets. Registration already rejects a
// rule with none, and a rule sets exactly one.
func (r Rule) verb() Verb {
	switch {
	case r.Check != nil:
		return VerbCheck
	case r.Rewrite != nil:
		return VerbRewrite
	case r.Gate != nil:
		return VerbGate
	case r.Advise != nil:
		return VerbAdvise
	default:
		return VerbDisplay
	}
}

// runnable reports whether a rule should execute for this event: right event,
// right tool, enabled, and its binaries present. A missing binary is reported
// by RunChecks as a blocking finding, so the other verbs just skip.
func (r Rule) runnable(ev *Event) bool {
	why := r.skipReason(ev)
	if why == "" {
		return true
	}
	// Only trace a rule that was plausibly meant for this event; every rule in
	// the registry is "wrong event" for most events, and saying so is noise.
	if r.handles(ev.HookEventName) {
		traceSkip(r, ev, why)
	}
	return false
}

// RunAdvice gathers advice from EVERY matching rule, not just the first:
// Claude Code delivers every hook's additionalContext, so short-circuiting
// would silently drop what the other rules said.
func RunAdvice(ev *Event) []Advice {
	var out []Advice
	for _, r := range registry {
		if r.Advise == nil || !r.runnable(ev) {
			continue
		}
		var a *Advice
		done := runWithTimeout(RuleTimeout(r.ID, r.Timeout), func(ctx context.Context) {
			a = r.Advise(ctx, ev)
		})
		if done && a != nil && strings.TrimSpace(a.Text) != "" {
			a.Rule = r.ID
			out = append(out, *a)
		}
	}
	return out
}

// CheckResults splits what a Check produced by how the user wants it treated:
// blocking findings, or the same text delivered as non-blocking context.
type CheckResults struct {
	Findings   []Finding
	Advisories []Advice
}

// RunChecks executes every matching Check rule, routing each rule's output by
// its configured severity. A rule that overruns its budget is abandoned
// fail-open, and a SingleFailure rule that fires short-circuits with its own.
func RunChecks(ev *Event) CheckResults {
	var res CheckResults
	for _, r := range registry {
		if !r.handles(ev.HookEventName) || !r.appliesTo(ev.ToolName) {
			continue
		}
		if !RuleEnabled(r.ID, r.EnabledByDefault) {
			continue
		}
		// Severity first. "off" has to mean exactly what disabling means, or a
		// silenced rule still blocks calls the moment its tool goes missing.
		sev := r.severity()
		if sev == SeverityOff {
			continue
		}
		// Every verb's missing binaries surface here, in the blocking channel:
		// an enabled check that cannot run is a failure, not a suggestion.
		if missing := MissingBinaries(r.needs()); len(missing) > 0 {
			res.Findings = append(res.Findings, missingBinaryFindings(r.ID, missing)...)
			continue
		}
		if r.Check == nil {
			continue
		}
		var fired []Finding
		started := time.Now()
		done := runWithTimeout(RuleTimeout(r.ID, r.Timeout), func(ctx context.Context) {
			for _, f := range r.Check(ctx, ev) {
				f.Rule = r.ID
				fired = append(fired, f)
			}
		})
		if !done {
			if r.FailClosedOnTimeout {
				traceRule(r, ev, VerbCheck, started, "TIMED OUT — reporting, fail-closed")
				res.Findings = append(res.Findings, timedOut(r))
				continue
			}
			traceRule(r, ev, VerbCheck, started, "TIMED OUT — abandoned, fail-open")
			continue
		}
		traceRule(r, ev, VerbCheck, started,
			fmt.Sprintf("%d finding(s), severity %s", len(fired), sev))
		if len(fired) == 0 {
			continue
		}
		if sev == SeverityAdvisory {
			// Same text, delivered as context instead of a refusal. An advisory
			// never short-circuits: it is not a diagnosis competing to be heard.
			res.Advisories = append(res.Advisories, Advice{Rule: r.ID, Text: advisoryText(fired)})
			continue
		}
		if r.SingleFailure {
			return CheckResults{Findings: fired, Advisories: res.Advisories}
		}
		res.Findings = append(res.Findings, fired...)
	}
	return res
}

// advisoryText flattens a rule's findings into one piece of context. The
// rendered table is the blocking format; an advisory is prose alongside the
// tool result.
func advisoryText(fired []Finding) string {
	lines := make([]string, 0, len(fired))
	for _, f := range fired {
		lines = append(lines, NormalizeProse(f.Message))
	}
	return strings.Join(lines, "\n")
}

// RunGates runs each matching Gate under its Timeout and returns the first
// non-nil Decision; defers (nil) and overruns (fail-open) fall through. Run it
// only after RunChecks found nothing, so a Check deny beats a gate allow.
func RunGates(ev *Event) *Decision {
	for _, r := range registry {
		if r.Gate == nil || !r.runnable(ev) {
			continue
		}
		var dec *Decision
		started := time.Now()
		done := runWithTimeout(RuleTimeout(r.ID, r.Timeout), func(ctx context.Context) {
			dec = r.Gate(ctx, ev)
		})
		if !done {
			if r.FailClosedOnTimeout {
				traceRule(r, ev, VerbGate, started, "TIMED OUT — blocking, fail-closed")
				return &Decision{
					Rule:       r.ID,
					Permission: failClosedVerdict(ev.HookEventName),
					Reason:     timedOut(r).Message,
				}
			}
			traceRule(r, ev, VerbGate, started, "TIMED OUT — abandoned, fail-open")
			continue
		}
		if dec != nil {
			dec.Rule = r.ID
			return dec
		}
	}
	return nil
}

// failClosedVerdict is the "do not proceed" decision for a gate that overran
// where NOT deciding is the failure. Vocabulary is per-event: tool gates deny,
// an elicitation gate cancels.
func failClosedVerdict(event EventName) string {
	switch event {
	case Elicitation, ElicitationResult:
		return ActionCancel
	default:
		return PermissionDeny
	}
}

// RunDisplays returns the first display rewrite for a MessageDisplay delta, or
// nil to leave the screen alone. Chaining is deliberately not supported: two
// rules rewriting the same text would fight over offsets.
func RunDisplays(ev *Event) *DisplayContent {
	for _, r := range registry {
		if r.Display == nil || !r.runnable(ev) {
			continue
		}
		var dc *DisplayContent
		done := runWithTimeout(RuleTimeout(r.ID, r.Timeout), func(ctx context.Context) {
			dc = r.Display(ctx, ev)
		})
		if done && dc != nil {
			dc.Rule = r.ID
			return dc
		}
	}
	return nil
}

// runWithTimeout runs fn under a deadline (0 = defaultRuleTimeout), giving it a
// ctx cancelled at the deadline. Returns false on timeout — results ignored
// (fail-open), goroutine abandoned (harmless IF fn wired ctx into any child).
func runWithTimeout(seconds int, fn func(ctx context.Context)) bool {
	if seconds <= 0 {
		seconds = defaultRuleTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(ctx)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

// RunRewrites returns the first input rewrite from a matching Rewrite rule, or
// nil if none applies. Callers apply it ONLY when RunChecks found nothing, so
// a denied event is never rewritten.
func RunRewrites(ev *Event) *Mutation {
	for _, r := range registry {
		if r.Rewrite == nil || !r.runnable(ev) {
			continue
		}
		var m *Mutation
		// Rewrite was the one verb running unbounded; a slow rewrite stalled the
		// tool call with nothing to abandon it.
		done := runWithTimeout(RuleTimeout(r.ID, r.Timeout), func(ctx context.Context) {
			m = r.Rewrite(ctx, ev)
		})
		if done && m != nil {
			m.Rule = r.ID
			return m
		}
	}
	return nil
}

func (r Rule) appliesTo(tool string) bool {
	return len(r.Tools) == 0 || slices.Contains(r.Tools, tool)
}
