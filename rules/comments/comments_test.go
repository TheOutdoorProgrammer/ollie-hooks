package comments

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func ccWrite(path, content string) *hook.Event {
	in, _ := json.Marshal(map[string]any{"file_path": path, "content": content})
	return &hook.Event{HookEventName: "PostToolUse", ToolName: "Write", ToolInput: in}
}

func ccEdit(path, oldS, newS string) *hook.Event {
	in, _ := json.Marshal(map[string]any{"file_path": path, "old_string": oldS, "new_string": newS})
	return &hook.Event{HookEventName: "PostToolUse", ToolName: "Edit", ToolInput: in}
}

func TestCheckCommentsWrite(t *testing.T) {
	hooktest.NoConfig(t) // no config file → built-in defaults (3 lines, 80 chars)
	long := "// " + strings.Repeat("x", 90)
	cases := []struct {
		name, content string
		fire          bool
		reason        string
	}{
		{"short", "// short helper\nfunc f(){}\n", false, ""},
		{"over-lines", "// a\n// b\n// c\n// d\nfunc f(){}\n", true, ">3 lines"},
		{"over-len", long + "\nfunc f(){}\n", true, ">80 chars"},
		{"memo", "// changed foo to bar\nfunc f(){}\n", true, "agent-memo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := checkComments(context.Background(), ccWrite("/tmp/x.go", c.content))
			if c.fire == (len(got) == 0) {
				t.Fatalf("fire=%v but got %d findings: %s", c.fire, len(got), hooktest.Messages(got))
			}
			if c.fire && !strings.Contains(hooktest.Messages(got), c.reason) {
				t.Errorf("want reason %q in: %s", c.reason, hooktest.Messages(got))
			}
		})
	}
}

func TestCheckCommentsEditScoping(t *testing.T) {
	hooktest.NoConfig(t)
	block := "// a\n// b\n// c\n// d\n" // a 4-line block that would fire if touched

	// The block is unchanged between old and new (only surrounding code changed)
	// → untouched → must NOT fire.
	if got := checkComments(context.Background(), ccEdit("/tmp/x.go", block+"code\n", block+"code2\n")); len(got) != 0 {
		t.Errorf("untouched pre-existing comment should not fire, got: %s", hooktest.Messages(got))
	}
	// The block is newly added → must fire.
	if got := checkComments(context.Background(), ccEdit("/tmp/x.go", "code\n", block+"code\n")); len(got) == 0 {
		t.Errorf("newly added >3-line comment should fire")
	}
}

func TestMemoGuidanceVariant(t *testing.T) {
	hooktest.NoConfig(t)
	got := checkComments(context.Background(), ccWrite("/tmp/x.go", "// changed foo to bar\nfunc f(){}\n"))
	if len(got) == 0 || !strings.Contains(got[0].Message, "AGENT-MEMO") {
		t.Errorf("memo case should lead with the AGENT-MEMO guidance: %s", hooktest.Messages(got))
	}
}

func TestSkipWhitelisted(t *testing.T) {
	skip := []scannedComment{
		{lines: []string{"# given"}},
		{lines: []string{"// noqa"}},
		{lines: []string{"#!/bin/bash"}},
	}
	for _, c := range skip {
		if !skipWhitelisted(c) {
			t.Errorf("should whitelist: %v", c.lines)
		}
	}
	keep := []scannedComment{
		{lines: []string{"// a real explanatory comment"}},
		{lines: []string{"# given", "# plus real prose"}}, // not every line is a marker
	}
	for _, c := range keep {
		if skipWhitelisted(c) {
			t.Errorf("should NOT whitelist: %v", c.lines)
		}
	}
}

func TestIsAgentMemo(t *testing.T) {
	memo := []string{"// changed foo to bar", "# now this works", "// this implements X", "// int -> string"}
	for _, m := range memo {
		if !isAgentMemo(scannedComment{lines: []string{m}}) {
			t.Errorf("should be memo: %q", m)
		}
	}
	notMemo := []string{"// returns the running total", "# the authenticated user id"}
	for _, m := range notMemo {
		if isAgentMemo(scannedComment{lines: []string{m}}) {
			t.Errorf("should NOT be memo: %q", m)
		}
	}
}
