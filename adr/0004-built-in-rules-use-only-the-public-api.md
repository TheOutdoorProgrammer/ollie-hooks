# 4. Built-in rules live in their own packages and use only the public API

## Status

Accepted.

## Context and problem statement

Everything started as `package main` in one flat directory.
That works until you want other people to write rules, and then it does not work at all: nobody can import `main`, so "write a rule in Go" is impossible and the only extension path left is out-of-process.

The obvious fix is to export a `hook` package.
The less obvious question is where the shipped rules should live, because leaving them inside `main` would let them keep reaching for internals that outsiders cannot touch.

## Considered options

1. **`hook` package, built-in rules stay in `main`.** Least moving.
2. **`hook` package, one package per built-in rule.**
3. **`hook` package with an `internal` escape hatch for built-ins.**

## Decision outcome

Option 2. Every built-in rule is its own package under `rules/`, importing `hook` exactly the way a stranger's rule would.

The point is that this is inconvenient on purpose.
A built-in that needs something unexported cannot have it, and that is the signal: the public API is incomplete, discovered at compile time rather than in a stranger's issue six months later.
Built-ins are the dogfood, not a privileged tier.

Option 3 was rejected for exactly that reason, an escape hatch means the built-ins stop being a test of the API the moment one of them uses it.

This has already paid for itself. `EditedFile` did not read `notebook_path` and did not resolve relative paths, so two rules had quietly grown their own private copies that did.
Those copies were the evidence the shared helper was too weak, and folding them back in fixed a silent bypass for every future rule that would have used it.

## Consequences

Good:

- An incomplete public API fails the build instead of shipping.
- A third-party rule and a built-in are written the same way, so the README examples are real.
- `hooktest` has to be good enough for the built-ins' own tests, which makes it good enough for everyone else's.

Bad:

- A directory and an import line per rule, and a genuinely tedious one-time move.
- Anything two rules share has to be exported from `hook`, which grows the public surface and the compatibility promise that comes with it.
- Rules must register explicitly rather than through `init()` side effects. More typing, but the enabled set stays greppable.
