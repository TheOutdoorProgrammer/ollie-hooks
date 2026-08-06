package bashoutput

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// bashResponse is the Bash tool's PostToolUse payload.
type bashResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// displayBashOutput re-prints a Bash result in full as a systemMessage.
// `verbose` would uncollapse tool output but also disables the MessageDisplay
// override the mermaid rules need; systemMessage is not gated by it.
func displayBashOutput(ctx context.Context, ev *hook.Event) *hook.DisplayContent {
	cfg := loadConfig()
	// A failed command fires PostToolUseFailure instead, with no tool_response:
	// `error` already holds the exit code plus the combined output.
	body, headerColor := bashBody(ev), cfg.CommandColor
	if ev.HookEventName == hook.PostToolUseFailure {
		headerColor = cfg.FailedColor
	}
	if strings.TrimSpace(body) == "" {
		return nil
	}
	if strings.Count(body, "\n")+1 < cfg.MinLines {
		return nil // short enough that Claude Code shows it anyway
	}
	// The leading newline stays outside the escape: the envelope swaps it for a
	// visible marker, and a colour code there would be swapped instead.
	out := "\n"
	if cmd := bashCommand(ev.ToolInput); cmd != "" {
		out += hook.Paint(cmd, headerColor) + "\n"
	}
	return &hook.DisplayContent{Text: out + hook.Paint(body, cfg.OutputColor)}
}

// bashBody is the text to echo: stdout+stderr on success, and on failure the
// `error` string, which already carries the exit code and combined output.
func bashBody(ev *hook.Event) string {
	if ev.HookEventName == hook.PostToolUseFailure {
		return strings.TrimRight(ev.Error, "\n")
	}
	var resp bashResponse
	if err := json.Unmarshal(ev.ToolResponse, &resp); err != nil {
		return ""
	}
	body := strings.TrimRight(resp.Stdout, "\n")
	if e := strings.TrimRight(resp.Stderr, "\n"); e != "" {
		if body != "" {
			body += "\n"
		}
		body += e
	}
	return body
}

// bashCommand renders the command that produced the output, so a screenful of
// text is attributable. Empty when the payload carries no command.
func bashCommand(toolInput json.RawMessage) string {
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(toolInput, &in); err != nil || strings.TrimSpace(in.Command) == "" {
		return ""
	}
	return "$ " + strings.TrimSpace(in.Command)
}
