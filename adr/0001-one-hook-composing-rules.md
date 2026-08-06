# 1. One hook process composing rules, not one hook per rule

## Status

Accepted.

## Context and problem statement

Claude Code runs hooks per event, and every matching hook is an independent process running in parallel.
That is fine while you have one.
It stops being fine the moment two of them want to influence the same tool call, because the hooks reference says so plainly:

> When multiple `PreToolUse` hooks return `updatedInput`, the last one to finish takes effect.
> Since hooks run in parallel, the order is non-deterministic.

So the platform gives you no way to say "the secret redactor runs before the formatter", or "if the policy check denied this, do not also rewrite it".
Whoever finishes last wins, and which one that is changes run to run.

## Considered options

1. **One hook entry per rule.** What everyone does by default.
2. **One hook entry, several rules composed inside it.**
3. **A long-running daemon that hooks talk to.**

## Decision outcome

Option 2. ollie-hooks registers as a single hook per event and runs the rules itself, in a defined order, with precedence living in exactly one function (`hook.Decide`).

Findings beat gates, gates beat rewrites, and a blocked call is never also silently rewritten.
That ordering is a property of the code rather than of process scheduling, so it is testable and it does not change between runs.

Option 3 was rejected because a hook already gets a whole process per event and the work is milliseconds.
A daemon would add lifecycle, staleness and a new failure mode to save nothing.

## Consequences

Good:

- Precedence is deterministic and lives in one readable place.
- One config file lists everything, so "what rules do I have" has an answer.
- Rules share the config loader, the state store, subprocess handling and the findings table instead of each re-solving them.
- N rules cost one process per event rather than N.

Bad:

- A crash in the composer takes every rule with it, so it fails open and exits 0 on any internal error.
- Rules run sequentially, so a slow rule delays the ones after it. Each has its own timeout for that reason.
- Users who want one rule still install the whole thing.
