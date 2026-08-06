package hook

import (
	"fmt"
	"os"
	"time"
)

// DebugEnv turns on the stderr trace.
const DebugEnv = "OLLIE_HOOKS_DEBUG"

// debugOn is read once at startup. The trace check sits inside the per-rule
// loop on MessageDisplay, which fires once per streamed delta, so it must not
// touch the environment each time.
var debugOn = os.Getenv(DebugEnv) != ""

// tracef writes to stderr, which Claude Code routes to its debug log on a
// zero exit. Rules fail open by design, so without this a rule that times out
// or never matches produces nothing at all to diagnose.
func tracef(format string, args ...any) {
	if !debugOn {
		return
	}
	fmt.Fprintf(os.Stderr, "ollie-hooks: "+format+"\n", args...)
}

// traceSkip records why a rule did not run. The silent skips are the ones
// people file bugs about: "my rule does nothing" with no way to see which
// condition rejected it.
func traceSkip(r Rule, ev *Event, why string) {
	tracef("skip %s on %s: %s", r.ID, ev.HookEventName, why)
}

// traceRule records a rule that ran, with how long it took. Timing matters on
// the tool path, where the whole hook sits in front of every call.
func traceRule(r Rule, ev *Event, verb Verb, started time.Time, outcome string) {
	tracef("ran %s (%s) on %s in %s: %s",
		r.ID, verb, ev.HookEventName, time.Since(started).Round(time.Microsecond), outcome)
}

// skipReason explains why runnable() rejected a rule, or "" when it did not.
func (r Rule) skipReason(ev *Event) string {
	if !r.handles(ev.HookEventName) {
		return "not registered for this event"
	}
	if !r.appliesTo(ev.ToolName) {
		return "tool " + ev.ToolName + " not in its list"
	}
	if !RuleEnabled(r.ID, r.EnabledByDefault) {
		return "disabled in config"
	}
	if missing := MissingBinaries(r.RequiresBinaries); len(missing) > 0 {
		return "missing binary " + missing[0].Bin
	}
	return ""
}

// timedOut is what a fail-closed rule reports when it overran. Naming the
// budget matters: the fix is usually raising it, not disabling the check.
func timedOut(r Rule) Finding {
	return Finding{Rule: r.ID, Message: `
		This check did NOT complete — it exceeded its ` +
		fmt.Sprint(RuleTimeout(r.ID, r.Timeout)) + `s budget and was abandoned.
		It is configured to fail closed, so the call is blocked rather than
		passing unchecked. Raise [rules.` + r.ID + `].timeout, or disable the
		rule if it is not worth the wait.`}
}
