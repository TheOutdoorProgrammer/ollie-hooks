# 3. Rules fail open, with an opt-in exception

## Status

Accepted.

## Context and problem statement

A hook sits in front of every tool call.
When one misbehaves — errors, hangs, or shells out to a tool that is not installed — something has to happen, and both answers are bad in different ways.

Fail closed and a broken rule blocks the session.
Fail open and a broken rule is invisible, which for a security rule means it stops protecting you without ever saying so.

## Considered options

1. **Fail closed everywhere.** Any rule that cannot complete blocks the call.
2. **Fail open everywhere.** Any rule that cannot complete is abandoned.
3. **Fail open by default, per-rule opt-out.**

## Decision outcome

Option 3.

The default is fail open, because the worst outcome for a tool that gates every action is wedging someone's session, and the person it wedges may not know ollie-hooks is what did it.

The exception is `FailClosedOnTimeout`, and `secret-scan` sets it.
For that rule the failure mode *is* the thing it exists to prevent: a prompt that went unscanned is indistinguishable from a clean one, so silence there is a false negative on a credential.

Two supporting rules follow from the same reasoning:

- A missing binary is always reported, never skipped. Enabling a rule is a request for it to run, so "your scanner is not installed" is information, not noise.
- The scanner's own failures are treated the same way. A non-zero exit or an unreadable report is a failed scan, not a clean one.

## Consequences

Good:

- No rule can take a session down, which is the promise that makes it reasonable to install this at all.
- The one rule where silence is dangerous does not have it.
- Missing tools surface as an actionable message rather than as nothing happening.

Bad:

- A broken rule is quiet by default, and "my rule does nothing" is the resulting support question. `OLLIE_HOOKS_DEBUG=1` exists specifically to answer it, printing every skip and its reason.
- Fail-open is a security posture the user has to understand for anything they write themselves, and third-party rule authors will have to be told.
