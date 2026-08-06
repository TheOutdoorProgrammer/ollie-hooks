package lint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func TestLinterFor(t *testing.T) {
	cases := []struct {
		path    string
		wantOK  bool
		wantCmd string
	}{
		{"/x/foo.md", true, "markdownlint-cli2"},
		{"/x/FOO.MD", true, "markdownlint-cli2"}, // extension match is case-insensitive
		{"/x/foo.py", true, "ruff"},
		{"/x/foo.sh", true, "shellcheck"},
		{"/x/foo.zsh", true, "zsh"},
		{"/x/foo.tf", true, "tofu"},
		{"/x/foo.tofu", true, "tofu"},
		{"/x/foo.lua", true, "selene"},
		{"/x/foo.go", true, "golangci-lint"},
		{"/x/foo.swift", true, "swiftlint"},
		{"/x/Dockerfile", true, "hadolint"},
		{"/x/api.Dockerfile", true, "hadolint"},
		{"/x/foo.rs", false, ""}, // no linter registered
		{"/x/README", false, ""}, // no extension, not a Dockerfile
	}
	for _, c := range cases {
		spec, ok := linterFor(c.path)
		if ok != c.wantOK {
			t.Errorf("linterFor(%q) ok = %v, want %v", c.path, ok, c.wantOK)
			continue
		}
		if ok && spec.cmd != c.wantCmd {
			t.Errorf("linterFor(%q) cmd = %q, want %q", c.path, spec.cmd, c.wantCmd)
		}
	}
}

// Dropping --strict silently disables Swift linting instead of breaking
// visibly: swiftlint exits 0 on a warning-only file, and exit code is this
// rule's found-problems signal.
func TestSwiftLintIsStrict(t *testing.T) {
	spec := lintersByExt[".swift"]
	if !slices.Contains(spec.args, "--strict") {
		t.Errorf("swiftlint args %v must include --strict", spec.args)
	}
}

func TestLinterForShebang(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		content string
		wantCmd string
	}{
		{"zsh.sh", "#!/bin/zsh\necho hi\n", "zsh"},            // zsh shebang in .sh -> zsh -n, not shellcheck
		{"envzsh.sh", "#!/usr/bin/env zsh\necho hi\n", "zsh"}, // env-style zsh shebang too
		{"bash.sh", "#!/bin/bash\necho hi\n", "shellcheck"},   // real bash stays on shellcheck
		{"none.sh", "echo hi\n", "shellcheck"},                // no shebang -> extension default
		{"zsh.bash", "#!/bin/zsh\necho hi\n", "zsh"},          // reroute applies to .bash as well
	}
	for _, c := range cases {
		p := filepath.Join(dir, c.name)
		if err := os.WriteFile(p, []byte(c.content), 0o644); err != nil {
			t.Fatal(err)
		}
		spec, ok := linterFor(p)
		if !ok || spec.cmd != c.wantCmd {
			t.Errorf("linterFor(%q) = %q,%v; want cmd %q", c.name, spec.cmd, ok, c.wantCmd)
		}
	}
	// A .sh path that doesn't exist can't be sniffed -> falls back to shellcheck.
	if spec, ok := linterFor(filepath.Join(dir, "missing.sh")); !ok || spec.cmd != "shellcheck" {
		t.Errorf("missing .sh should fall back to shellcheck, got %q", spec.cmd)
	}
}

func TestEditedFilePath(t *testing.T) {
	abs := editedFilePath(&hook.Event{ToolInput: json.RawMessage(`{"file_path":"/a/b/c.md"}`)})
	if abs != "/a/b/c.md" {
		t.Errorf("absolute path = %q", abs)
	}
	rel := editedFilePath(&hook.Event{CWD: "/root", ToolInput: json.RawMessage(`{"file_path":"sub/c.md"}`)})
	if rel != "/root/sub/c.md" {
		t.Errorf("relative path resolved against CWD = %q", rel)
	}
	// Fail-open: unreadable input or missing path yields "".
	for _, in := range []string{``, `{}`, `{"file_path":""}`, `not json`} {
		if p := editedFilePath(&hook.Event{ToolInput: json.RawMessage(in)}); p != "" {
			t.Errorf("editedFilePath(%q) = %q, want empty", in, p)
		}
	}
}

func TestFilterLines(t *testing.T) {
	// A markdownlint-cli2-style run: banner + summary must be dropped, only the
	// file:line issue lines kept.
	out := strings.Join([]string{
		"markdownlint-cli2 v0.23.0 (markdownlint v0.x)",
		"Finding: /x/foo.md",
		"Linting: 1 file(s)",
		"/x/foo.md:5:22 MD009/no-trailing-spaces Trailing spaces",
		"/x/foo.md:9 MD047/single-trailing-newline",
		"Summary: 2 error(s)",
		"",
	}, "\n")
	got := filterLines(out, false)
	if len(got) != 2 {
		t.Fatalf("filtered issues = %d (%v), want 2", len(got), got)
	}
	for _, g := range got {
		if !strings.HasPrefix(g, "/x/foo.md:") {
			t.Errorf("kept a non-issue line: %q", g)
		}
	}

	// raw mode (tofu fmt diff): every non-empty line survives, even without a
	// :line: locator.
	diff := "--- old/main.tf\n+++ new/main.tf\n@@ -1 +1 @@\n-foo=1\n+foo = 1\n\n"
	if raw := filterLines(diff, true); len(raw) != 5 {
		t.Errorf("raw lines = %d (%v), want 5", len(raw), raw)
	}
}

func TestKeepFile(t *testing.T) {
	// A package-scoped run reports findings across sibling files; keepFile must
	// keep only the edited file's, regardless of a leading ./ or path prefix.
	lines := []string{
		"rule_lint.go:70:20: undefined: Event (typecheck)",
		"./rule_lint.go:98:21: undefined: Finding (typecheck)",
		"event.go:5:1: some real issue",
		"sub/other.go:3: another package file",
	}
	got := keepFile(lines, "rule_lint.go")
	if len(got) != 2 {
		t.Fatalf("kept = %d (%v), want 2", len(got), got)
	}
	for _, g := range got {
		if !strings.Contains(g, "rule_lint.go:") {
			t.Errorf("kept a line for another file: %q", g)
		}
	}
}

// TestCheckLintPkgScoped verifies a package-scoped linter's findings are
// narrowed to the edited file, so sibling-file lines never leak through.
func TestCheckLintPkgScoped(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	dir := t.TempDir()
	// Fake linter ignores its args and prints findings for two files.
	script := filepath.Join(dir, "fakepkg")
	body := "#!/bin/sh\necho \"sample.fixture:3:1: FIX001 edited-file problem\"\n" +
		"echo \"sibling.fixture:9:1: FIX002 sibling problem\"\nexit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	lintersByExt[".fixture"] = linterSpec{name: "fakepkg", cmd: script, pkgScoped: true}
	defer delete(lintersByExt, ".fixture")

	target := filepath.Join(dir, "sample.fixture")
	if err := os.WriteFile(target, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]string{"file_path": target})
	advice := hook.RunAdvice(&hook.Event{HookEventName: "PostToolUse", ToolName: "Write", ToolInput: input})
	if len(advice) == 0 {
		t.Fatal("package-scoped linter produced no advice")
	}
	text := advice[0].Text
	if strings.Contains(text, "sibling problem") {
		t.Errorf("sibling-file issue leaked through: %q", text)
	}
	if !strings.Contains(text, "edited-file problem") {
		t.Errorf("edited-file issue missing: %q", text)
	}
}

func TestMarkdownlintConfigArgs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Absent config → no --config (run with defaults, don't error).
	if a := markdownlintConfigArgs(); a != nil {
		t.Errorf("with no config file, args = %v, want nil", a)
	}
	// Present config → --config <path>.
	cfg := filepath.Join(home, ".markdownlint-cli2.jsonc")
	if err := os.WriteFile(cfg, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := markdownlintConfigArgs()
	if len(got) != 2 || got[0] != "--config" || got[1] != cfg {
		t.Errorf("args = %v, want [--config %s]", got, cfg)
	}
}

// TestCheckLintEndToEnd exercises the full pipeline with a fake linter script,
// so the test never depends on any real linter being installed.
func TestCheckLintEndToEnd(t *testing.T) {
	hooktest.Config(t, RuleID, "enabled = true")
	dir := t.TempDir()

	// Fake linter: prints one file:line issue and exits non-zero.
	script := filepath.Join(dir, "fakelint")
	body := "#!/bin/sh\necho \"$1:3:1: FAKE001 a fake problem\"\nexit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	// Register it for a throwaway extension, cleaned up after.
	lintersByExt[".fixture"] = linterSpec{name: "fakelint", cmd: script}
	defer delete(lintersByExt, ".fixture")

	target := filepath.Join(dir, "sample.fixture")
	if err := os.WriteFile(target, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]string{"file_path": target})
	ev := &hook.Event{HookEventName: "PostToolUse", ToolName: "Write", ToolInput: input}

	advice := hook.RunAdvice(ev)
	if len(advice) != 1 {
		t.Fatalf("expected one piece of advice, got %d: %v", len(advice), advice)
	}
	if advice[0].Rule != "lint" {
		t.Errorf("rule = %q, want lint", advice[0].Rule)
	}
	if !strings.Contains(advice[0].Text, "fake problem") {
		t.Errorf("issue text missing: %q", advice[0].Text)
	}

	// Clean file (exit 0) → no findings.
	clean := filepath.Join(dir, "cleanlint")
	if err := os.WriteFile(clean, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	lintersByExt[".fixture"] = linterSpec{name: "cleanlint", cmd: clean}
	if f := hook.Run(ev); f != nil {
		t.Errorf("clean linter should yield no findings, got %v", f)
	}
}

func TestCheckLintFailOpen(t *testing.T) {
	mk := func(path string) *hook.Event {
		input, _ := json.Marshal(map[string]string{"file_path": path})
		return &hook.Event{HookEventName: "PostToolUse", ToolName: "Write", ToolInput: input}
	}
	// Unknown extension → no linter → nil.
	if f := adviseLint(context.Background(), mk("/tmp/whatever.rs")); f != nil {
		t.Errorf("unknown ext should pass, got %v", f)
	}
	// Known ext but linter not on PATH → fail-open nil.
	orig := lintersByExt[".fixture"]
	lintersByExt[".fixture"] = linterSpec{name: "nope", cmd: "definitely-not-a-real-binary-xyz"}
	defer func() {
		if orig.cmd == "" {
			delete(lintersByExt, ".fixture")
		} else {
			lintersByExt[".fixture"] = orig
		}
	}()
	if f := adviseLint(context.Background(), mk("/tmp/x.fixture")); f != nil {
		t.Errorf("missing linter binary should fail open, got %v", f)
	}
	// No path at all → nil.
	if f := adviseLint(context.Background(), &hook.Event{HookEventName: "PostToolUse", ToolName: "Write"}); f != nil {
		t.Errorf("no input should fail open, got %v", f)
	}
}
