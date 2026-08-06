# Writing a custom rule

A custom rule runs your own code on a hook event, in any language, without touching the ollie-hooks binary.
It is a separate program that reads a request on stdin and writes a reply on stdout — or an HTTP server that answers the same shapes.
ollie-hooks composes it with the built-in rules under the same precedence, so a custom Check can deny a call before a Rewrite ever runs.

## Enable it

Declare the rule under `[custom_rules.<id>]` in your config (`~/.config/ollie-hooks/config.toml`):

```toml
[custom_rules.no-curl]
enabled     = true
startup_cmd = "python3 ~/rules/policy.py"   # OR: server_url = "http://127.0.0.1:9000"
verb        = "Check"                        # Check | Advise | Gate | Rewrite | Display
events      = ["PreToolUse"]
tools       = ["Bash"]                       # omit for every tool
# timeout   = 30                             # seconds; optional
```

`verb` is a capability, not a label.
It decides which field of your reply is read, so a rule you enabled to *advise* can never start *denying* your tool calls after an update you did not review — anything else it returns is ignored.

`startup_cmd` runs the program once per event: request on stdin, reply on stdout, exit.
It is not run through a shell, so there are no pipes, redirects, or variable expansion; a leading `~` is expanded and quoted arguments hold together.
Use `server_url` instead for a rule that needs warm state, like a loaded index or a language server — same wire format, over HTTP POST.

One endpoint can serve several rules.
Give each its own `[custom_rules.<id>]` with the same `startup_cmd`, and ollie-hooks calls the program once per event, naming which of its rules to run.

`severity` and `timeout` for a custom rule are read from the same-id `[rules.<id>]` section, not `[custom_rules.<id>]` — so a custom Check can be downgraded to `advisory` just like a built-in.

## The wire protocol

**Request** (stdin) is the event exactly as Claude Code sent it, plus which of this endpoint's rules to run:

```json
{
  "event": { "hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": { "command": "curl x" } },
  "rules": ["no-curl"]
}
```

**Reply** (stdout) carries the fields your rules produce.
Only the field matching each rule's declared `verb` is read:

| verb | field | shape |
| --- | --- | --- |
| `Check` | `findings` | `[{ "message": "..." }]` — each becomes a blocking finding |
| `Advise` | `advice` | a string, delivered to the model as context, never blocking |
| `Gate` | `decision` | `{ "permission": "allow" \| "deny", "reason": "...", "updatedInput": { ... } }` |
| `Rewrite` | `mutation` | `{ "updatedInput": { ... } }`, or `{ "updatedOutput": ..., "note": "..." }` |
| `Display` | `display` | a string that replaces the on-screen text |

A `Rewrite`'s `updatedInput` (and a `Gate`'s) replaces the **whole** tool input, so start from the decoded input and hand back every field you did not change, or the ones you omit are dropped.

A single-rule endpoint can answer with the bare response.
One serving several rules keys them by id:

```json
{ "rules": { "no-curl": { "findings": [{ "message": "curl is not allowed here." }] } } }
```

An empty reply, or `{}`, means "nothing to say" — the rule fires nothing.

## Two ways to build it

**Raw JSON, any language.** Read stdin, write stdout:

```python
import json, sys

req = json.load(sys.stdin)
cmd = req["event"].get("tool_input", {}).get("command", "")
findings = [{"message": "curl is not allowed here."}] if "curl" in cmd else []
print(json.dumps({"rules": {"no-curl": {"findings": findings}}}))
```

**Typed, in Go.** Import the framework's own wire types so the shapes cannot drift:

```go
import "github.com/TheOutdoorProgrammer/ollie-hooks/hook"

func main() {
	var req hook.PluginRequest
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	var ev hook.Event
	_ = json.Unmarshal(req.Event, &ev)

	reply := hook.PluginReply{Rules: map[string]hook.PluginResponse{}}
	for _, id := range req.Rules {
		if id == "no-curl" {
			if cmd, _ := ev.Command(); strings.Contains(cmd, "curl") {
				reply.Rules[id] = hook.PluginResponse{
					Findings: []hook.Finding{{Message: "curl is not allowed here."}},
				}
			}
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(reply)
}
```

`hook.PluginRequest`, `hook.PluginResponse`, `hook.PluginReply`, and the field types (`hook.Finding`, `hook.Mutation`, `hook.Decision`, `hook.Event`) are all exported for exactly this — so a Go plugin never hand-builds the JSON or guesses a key.

## Rules of the road

Your rule **fails open**: if the program errors, times out, or is missing, ollie-hooks abandons it rather than wedge the session.
A rule whose *not running* is itself the failure — a scan that must happen — should say so through a Check finding, not lean on silence.

**Never echo a secret** back into a finding, note, or reply — whatever you return lands in the transcript, which is the thing a secret rule exists to keep it out of.

The `event` is the raw payload.
Decode `tool_input` and `tool_response` yourself, since their shape is per-tool.

See [adr/0002](adr/0002-json-over-stdio-for-out-of-process-rules.md) for why the transport is JSON over stdio rather than a supervised daemon or gRPC.
