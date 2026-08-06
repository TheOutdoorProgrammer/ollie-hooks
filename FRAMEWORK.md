# Framework design

Answers to the questions raised in review, plus the helper set.
Comment inline.

## 1. Should this be a package? Should rules live in their own packages?

**Yes to both, and the second one is the important one.**

Today everything is `package main` in a flat directory.
A third party cannot import `main`, so as long as that holds, "write your own rule in Go" is impossible — the only extension path left is the out-of-process one.
That alone forces the split.

```text
ollie-hooks/
├── cmd/ollie-hooks/      thin CLI: read stdin, dispatch, write stdout
├── hook/                 THE public API — everything a rule author touches
│   ├── event.go          Event + typed accessors
│   ├── rule.go           Rule, Finding, Decision, Mutation, DisplayContent
│   ├── registry.go       Register, Run, RunGates, RunDisplays, RunRewrites
│   ├── config.go         LoadConfig
│   ├── state.go          State store
│   ├── toon.go           findings table + wrapping
│   ├── ansi.go           colour
│   ├── exec.go           guarded subprocess
│   ├── paths.go          config/state dirs
│   └── hooktest/         fixtures so rule authors can test
├── rules/                built-ins, one package each
│   ├── lint/
│   ├── comments/
│   ├── prosewrap/
│   ├── bashoutput/
│   ├── mermaid/
│   └── nocodegen/
└── plugin/               gRPC + exec transports
```

**One package per rule, specifically because it is inconvenient.**
A built-in rule in its own package can only use what `hook/` exports.
The moment `lint` needs something unexported, that is proof the public API is incomplete — and we find out at compile time instead of when a stranger files an issue.
Built-ins become the dogfood, not a privileged tier.

The cost is real: ~6 new directories and an import line per rule.
Worth it. The alternative is discovering the API is insufficient after publishing it.

## 2. Should the config be one file or many?

**One file.** Four `*.toml` files was right when rules were personal and nobody had to discover them; it is wrong for opt-in.

If everything ships disabled, the very first thing a user needs is *the list of what exists*.
Four files means that list lives only in the README, so the config and the docs drift.
One file makes `ollie-hooks init` emit a commented catalogue you read top to bottom, and it gives `custom_rules` an obvious home.

```toml
# ~/.config/ollie-hooks/config.toml

[rules.lint]
enabled = true

[rules.comments]
enabled = true
max_comment_lines = 3
max_line_length   = 80

[rules.mermaid-stream]
enabled   = true
width_cap = 120
binary    = "mermaid-ascii"

[custom_rules.my-policy]
startup_cmd = "python3 ~/rules/my_policy.py"   # ollie-hooks owns the process

[custom_rules.company-guard]
server_url = "localhost:9000"                  # already running
```

`LoadConfig` then becomes one call instead of the 15-line loader every rule currently copies:

```go
cfg := DefaultConfig()
hook.LoadConfig("comments", &cfg)   // decodes [rules.comments] over the defaults
```

Migration note: this breaks the four existing files on your machine.
Given the only user is you, I would just break it rather than carry a compatibility shim.

## 3. State — the ask from review

```go
st := hook.RuleState("mermaid-stream").Scoped(ev.SessionID, ev.MessageID)

var buf mermaidState
if st.Load(&buf) { /* resume */ }
st.Save(&buf)
st.Clear()
```

`RuleState` namespaces by rule so two rules cannot collide.
`Scoped` hashes its parts into the filename — that is the `sha256` dance currently open-coded in `mermaidStatePath`, done once in the framework.
Files land in the state dir, never the config dir: this is regenerable scratch nobody edits, and a config directory filling with JSON is a bug.

Sweeping stays a framework concern.
`os.TempDir()` gave us orphan cleanup for free and moving out of it lost that, so `State` sweeps its own namespace on `Clear`.

## 4. TOON auto-wrapping

Today `expandFindings` splits on explicit newlines and pads continuation rows with `↳`.
Rule authors have to hand-break their own messages, which is why several messages in this repo are written as `oneline(...)` raw strings with manual phrasing.

The framework should wrap:

```go
return []hook.Finding{{Message: "one long sentence, however long it wants to be"}}
```

and the encoder breaks at a width with the `↳` marker already handled.

**One constraint worth knowing:** hooks run with no controlling terminal, so we cannot detect terminal width.
It has to be a config key with a sane default (100), not autodetection.

## 5. ANSI tooling

`ansi.go` has `paint()`. What is missing:

- **Named palette constants** so rules stop hardcoding hex. `hook.Red`, `hook.Green`, `hook.Muted` — retheming then happens in one place.
- **`NO_COLOR` support.** It is a standard, we ignore it, and that is a bug for anyone piping output.
- **Capability detection** via `COLORTERM`/`TERM` — truecolor, 256, or none.
- **`hook.Strip(s)`** to measure a painted string. This has already bitten us: `renderMermaid` counts runes for the width cap, and would be wrong the moment the body carried escapes.

## 6. Things nobody asked for that I would add

Ranked by how much duplication they remove.

**Typed Event accessors — biggest win.**
Every rule opens with the same unmarshal boilerplate against its own anonymous struct.

```go
path, ok := ev.FilePath()      // handles file_path AND notebook_path
cmd,  ok := ev.Command()       // Bash
out,  ok := ev.BashResult()    // stdout+stderr, or Error on the failure event
```

Getting `notebook_path` wrong is currently a silent bypass of any path-based rule — exactly the kind of thing a framework should make impossible.

**Guarded exec.**
`lint` and `mermaid` both do LookPath → exec → fail-open-on-missing, with the context timeout wired in.
One helper, one place to get the cancellation right:

```go
out, ok := hook.Run(ctx, "shellcheck", "-f", "gcc", path)   // ok=false if absent
```

**Touched-region diffing.**
`comments` and `prose-wrap` both need "what did this edit actually change", and it is the hardest part of writing a good PostToolUse rule.
Right now it exists twice.
Shipping it means a stranger can write a scoped rule without re-deriving old→new diffing.

**`hooktest`.**
The README says tests are REQUIRED for every rule.
That is unenforceable for third parties unless we hand them fixtures:

```go
ev := hooktest.PreToolUse("Write", map[string]any{"file_path": "/tmp/x.go"})
hooktest.AssertDenied(t, myRule, ev)
```

**Debug tracing.**
`OLLIE_HOOKS_DEBUG=1` printing which rules matched, which fired, which timed out, and how long each took.
Rules fail open on timeout, which means a broken rule is *silent* — that is the correct behaviour and a miserable debugging experience.
Plugin authors will need this on day one, and so will we when a gRPC rule hangs.

**Path matching.**
`nocodegen` repo patterns, `prose-wrap` ignore paths and `lint` extension dispatch are three variations on the same thing.
One matcher, glob or substring, used by all three and available to plugins.

## Open questions

1. **Does the restructure happen before or after the plugin API?**
   I would do it first — the package split is what makes a Go plugin API expressible at all, and doing it after means moving every file twice.
2. **Does `hook/` version independently?**
   If third parties import it, its API is a compatibility promise. A `v0` while it settles is honest.
3. **Do built-in rules register themselves via `init()`, or does `cmd/` wire them explicitly?**
   Explicit wiring is uglier but makes the enabled set greppable and kills import-for-side-effect. I lean explicit.
4. **How much of this lands before public?**
   Everything in §3-§5 is small. §1 is a day of mechanical moving. §6 is where the real time goes, and `hooktest` plus debug tracing are the two I would not ship without.
