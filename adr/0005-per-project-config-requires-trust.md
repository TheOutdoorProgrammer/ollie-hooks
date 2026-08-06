# 5. Per-project config requires trust, and can never weaken the secret rules

## Status

Accepted.

## Context and problem statement

Teams want ollie-hooks configured per repo, not just per user.
The headline case is `no-codegen`: a repo that refuses AI-authored code wants its protected trees committed alongside the code, so every clone enforces them without each person editing their own `~/.config`.
Linter choices and custom rules have the same pull.

But a config file that travels with a repo is a config file a stranger wrote.
A freshly cloned repo is untrusted code, and its `.ollie-hooks.toml` is untrusted input.
Two things make that dangerous:

1. **Custom rules run commands.** `[custom_rules.<id>].startup_cmd` is arbitrary local code execution. A repo that can register one owns your machine the moment you open it.
2. **Weakening is silent.** A project config that disables `secret-scan`, or downgrades it to advisory, or repoints its scanner binary, turns off a security control without anyone choosing that. A pasted credential then sails through, and nothing says the guard was removed.

So the question is not "should project config exist" but "under what authority does an untrusted file get to change what the gate does".

## Considered options

1. **Read project config unconditionally.** Simple, and a supply-chain hole: cloning a repo is enough to disable your secret scanner or run a command.
2. **Only-tighten merge.** Project config may enable a rule, escalate severity, add protected paths — never disable, downgrade, or run code. Safe, but "tighten" is per-field and per-rule: adding a `prose-wrap` ignore path *loosens*, a shorter timeout can make a fail-open rule skip. The monotonicity direction is bespoke to every key, which is a lot of security-critical special-casing that is easy to get subtly wrong.
3. **Trust acknowledgement, mirroring Claude Code.** Project config is ignored until the user explicitly trusts that project root. A trusted repo may tune the pure-config rules; an untrusted one does nothing but print a one-time notice. Plus a hard floor: no repo, trusted or not, may touch the secret rules or make ollie-hooks run a program — custom rules and binary repointing stay in user config.

## Decision outcome

Option 3.

**Discovery.** The nearest `.ollie-hooks.toml` walking up from the working directory is the project config. One file, like the user config.

**Trust is the gate.** Project config is applied only when its directory is in the user's trust list (`trust.toml` under the config dir). Until then it is ignored and a one-time stderr notice says it exists and how to trust it. This is the same posture as Claude Code trusting a folder: the default for unknown code is that it cannot act, and the human opts in per repo. `ollie-hooks trust` / `untrust` manage the list.

**Merge is section-level, and config only.** When trusted, a `[rules.<id>]` the project defines replaces the user's section for that id; ids it does not mention keep the user's config. Whole-section replacement is predictable — a rule is configured by exactly one of the two files — and avoids the per-field merge ambiguity that sank option 2.

**Executable config is a hard floor, not just the secret rules.** `secret-scan` and `secret-redact` take config from the user file only, and so does any rule that runs an external program — the scanner rules, the linter, the mermaid renderer. Project config, trusted or not, cannot disable the secret rules, downgrade them, or repoint any rule's binary at an arbitrary command. Trusting a repo says "run its rules"; it never says "run this program". And `[custom_rules.*]` in a project file is ignored outright, with a notice: a `startup_cmd` is arbitrary local code, and a trust granted once must not become a standing execution channel that a later commit or merged PR can quietly repoint. Code execution stays in the user's own config, written by the user, on the user's machine.

### Consequences

Good:

- Cloning a hostile repo changes nothing until the user acts, and even then the code-execution paths stay shut. A project `startup_cmd` never runs, and no rule's binary can be repointed from project scope; the silent-weakening vector is closed by default and code execution is closed always.
- The primary use case works: trust your own repo once, and its `no-codegen` trees and pure-config rule choices travel with it.
- Whole-section semantics are simple to reason about and to test.
- The floor means even a mistaken trust cannot remove credential scanning or hand a repo a way to run a program.

Bad:

- Trust is per-project friction: a new repo's config does nothing until acknowledged, which will surprise someone who expected it to just work. The one-time notice is the mitigation.
- Whole-section replacement means a project cannot tweak one field of a rule and inherit the rest — it restates the section. For a small config this is fine; a larger one duplicates keys.
- Trust is coarse: trusting a repo trusts its whole config (minus the floor), not a reviewed subset. This is the same trade Claude Code makes, and the floor limits the blast radius.
