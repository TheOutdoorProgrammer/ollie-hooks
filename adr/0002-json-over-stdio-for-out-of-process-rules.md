# 2. JSON over stdio for out-of-process rules, not gRPC

## Status

Accepted. Supersedes the gRPC sketch in the original framework notes.

## Context and problem statement

Rules written in Go can be compiled in.
Everyone else needs a way to write a rule in whatever language they already use, which means a wire protocol.

The first design was gRPC: a `.proto`, generated stubs, and a plugin standing up a server that ollie-hooks talks to.

## Considered options

1. **gRPC.** Typed contract, generated clients in every language, streaming if it were ever wanted.
2. **JSON on stdin and stdout.** The plugin reads a request, writes a reply, exits.
3. **HTTP with JSON.** A plugin that is already a server.

## Decision outcome

Option 2 as the default, with option 3 available via `server_url` for a plugin that needs warm state (a loaded index, a live language server).

The deciding argument was what each option costs the person writing a plugin.
A gRPC rule means installing protoc, generating stubs, and depending on a gRPC runtime before writing a line of policy.
A stdio rule is this, complete:

```python
import json, sys
req = json.load(sys.stdin)
print(json.dumps({"findings": [{"message": "no"}]}))
```

Two secondary arguments pointed the same way.
gRPC would have taken ollie-hooks from three dependencies to twenty-odd, in a binary that gates every tool call and should be easy to audit.
And the event is already JSON — Claude Code hands it over as JSON, so a plugin gets exactly the payload it would have read writing a hook by hand, with no schema in between to drift.

## Consequences

Good:

- A plugin in any language is a few lines, and needs no build step or code generation.
- The dependency count stays small enough to read.
- The request body is the hook event verbatim, so plugin authors can use the official hooks documentation directly.
- No daemon to supervise: a process per event, matching how hooks already work.

Bad:

- No typed contract. A plugin that returns the wrong shape is ignored rather than rejected at compile time, which is why `verb` exists as a capability declaration and why `OLLIE_HOOKS_DEBUG` traces every plugin call.
- Process startup per event. Rules sharing one endpoint are batched into a single call to keep that to one spawn rather than one per rule.
- A plugin needing warm state has to run its own server and be pointed at with `server_url`.
