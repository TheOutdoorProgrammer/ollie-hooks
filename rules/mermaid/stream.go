package mermaid

import (
	"context"
	"strings"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// mermaidState is the per-message reassembly buffer. Deltas split at arbitrary
// offsets, so Carry holds a trailing partial line: treating a fragment as a
// whole line splits a node in two and silently renders the wrong diagram.
type mermaidState struct {
	InFence bool     `json:"in_fence"`
	Body    []string `json:"body"`
	Raw     []string `json:"raw"`
	Carry   string   `json:"carry"`
}

// displayMermaidStream swaps ```mermaid fences for a render as text streams.
// The render persists — unless `verbose` is on, which disables it (adr/0003).
func displayMermaidStream(ctx context.Context, ev *hook.Event) *hook.DisplayContent {
	cfg := loadConfig()
	if ev.MessageID == "" {
		return nil
	}
	// Scoped per session+message so concurrent sessions, which all share this
	// one global hook, never read each other's buffer.
	store := hook.RuleState(RuleID).Scoped(ev.SessionID, ev.MessageID)
	st := &mermaidState{}
	store.Load(st)
	idle := !st.InFence && st.Carry == ""
	if idle && !strings.Contains(ev.Delta, "`") {
		return nil
	}

	text, out := st.Carry+ev.Delta, &strings.Builder{}
	st.Carry = ""
	lines := splitKeepEnds(text)
	if n := len(lines); n > 0 && !strings.HasSuffix(lines[n-1], "\n") {
		st.Carry = lines[n-1]
		lines = lines[:n-1]
	}
	for _, line := range lines {
		st.feed(line, out, cfg)
	}
	if ev.Final {
		if st.Carry != "" {
			st.feed(st.Carry, out, cfg)
			st.Carry = ""
		}
		// An unterminated fence would vanish with the state file — flush it raw.
		if st.InFence {
			out.WriteString(strings.Join(st.Raw, ""))
			st = &mermaidState{}
		}
		store.Clear()
	} else {
		_ = store.Save(st)
	}
	return &hook.DisplayContent{Text: out.String()}
}

// feed consumes one complete line (its newline kept) of the message.
func (st *mermaidState) feed(line string, out *strings.Builder, cfg config) {
	trimmed := strings.TrimSpace(line)
	if !st.InFence {
		if strings.HasPrefix(trimmed, mermaidFenceOpen) {
			st.InFence, st.Body, st.Raw = true, nil, []string{line}
		} else {
			out.WriteString(line)
		}
		return
	}
	st.Raw = append(st.Raw, line)
	if trimmed != mermaidFenceClose {
		st.Body = append(st.Body, strings.TrimRight(line, "\n"))
		return
	}
	st.InFence = false
	if art := renderMermaid(st.Body, cfg); art != "" {
		out.WriteString(mermaidFenceClose + "\n" + art + "\n" + mermaidFenceClose + "\n")
	} else {
		out.WriteString(strings.Join(st.Raw, "")) // unsupported or too wide
	}
	st.Body, st.Raw = nil, nil
}

// splitKeepEnds splits into lines while keeping each newline, so rejoining is
// byte-identical — the screen must not gain or lose whitespace.
func splitKeepEnds(s string) []string {
	var lines []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			lines = append(lines, s)
			break
		}
		lines = append(lines, s[:i+1])
		s = s[i+1:]
	}
	return lines
}
