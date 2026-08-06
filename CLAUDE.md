# ollie-hooks

A rule engine for Claude Code hooks: one hook that composes many rules with deterministic precedence.
See [README.md](README.md) for what it is, and [CUSTOM_RULES.md](CUSTOM_RULES.md) for writing an out-of-process rule.

## Build and test

- `make test` runs the suite with the race detector.
- `make install` builds to `~/bin/ollie-hooks`, but only after vet and the tests pass.
- `make docs` regenerates `docs/rules.md` from the registered rules.
- `make build` produces a local `ollie-hooks` binary.

`docs/rules.md` is generated, never hand-edited.
If a rule's config or description changes, run `make docs` and commit the result.

## Layout

- `hook/` is the public API: everything a rule author can import.
- `rules/<name>/` is one built-in rule each, with `register.go`, `config.go`, and the logic beside them.
- `hook/hooktest/` has fixtures for testing a rule without a running Claude Code.
- `cmd/ollie-hooks/` is the CLI entrypoint.
- `adr/` holds the decisions worth not re-litigating.

## Conventions

Built-in rules use only the public `hook` API, the same surface a third party gets (adr/0004).
If a built-in needs something unexported, that is the signal to export it, not to reach around the wall.

Rules fail open: one that errors or overruns its timeout is abandoned, never allowed to wedge a session (adr/0003).
The single exception is `FailClosedOnTimeout`, which `secret-scan` sets.

Registration is loud: a misspelled event, or a verb an event cannot express, panics at startup rather than firing never.

This repo eats its own dog food, so match the house style the rules enforce:

- Keep comments terse, and explain the why rather than the what.
A block over three lines, or a line over eighty characters, is a smell.
- One sentence per line in markdown, and let the renderer wrap it.
- No em-dashes. A comma, colon, period, or parentheses does the job.
