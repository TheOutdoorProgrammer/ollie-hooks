package lint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// linterSpec is how to invoke one linter on a single file. The tool must exit
// non-zero when it finds problems (the near-universal convention) — that exit
// code is the signal, and the matching output lines become findings.
type linterSpec struct {
	name    string          // short label shown in the finding
	cmd     string          // executable, resolved on PATH
	args    []string        // fixed args placed before the file path
	dynArgs func() []string // args computed at run time (nil if none)
	raw     bool            // true: surface every output line (diff-style tools like tofu fmt); false: keep only file:line issue lines and drop banners
	// fromRepoRoot runs the linter from the file's git repo root instead of the
	// file's own directory. markdownlint-cli2 only walks config discovery up to
	// the cwd, so a repo-local .markdownlint-cli2.jsonc at the repo root never
	// reaches files in subdirectories unless we start from the root.
	fromRepoRoot bool
	// configFile runs the linter from the nearest ancestor directory that
	// contains a file of this name (walking up from the edited file's dir).
	// selene (≤0.31) reads selene.toml only from its cwd and never searches
	// parents, so a config at a project root never reaches files in subdirs
	// unless the run starts there. Unlike fromRepoRoot this keys off the config
	// file itself, so it works in trees that aren't git repos (e.g. ~/.config/nvim).
	configFile string
	// pkgScoped lints the file's whole directory (package) instead of just the
	// one file, then narrows the findings back to the edited file. Required for
	// package-aware tools like golangci-lint: handed a single .go file they
	// typecheck it in isolation and report every symbol defined in a sibling
	// file as "undefined" — pure false positives that drown the real findings.
	pkgScoped bool
}

// lintersByExt maps a lowercased extension (with dot) to its linter. Adding a
// language is one line. A linter absent from PATH is skipped, so entries for
// not-yet-installed tools are harmless — they light up when the binary appears.
var lintersByExt = map[string]linterSpec{
	".md":       {name: "markdownlint", cmd: "markdownlint-cli2", dynArgs: markdownlintConfigArgs, fromRepoRoot: true},
	".markdown": {name: "markdownlint", cmd: "markdownlint-cli2", dynArgs: markdownlintConfigArgs, fromRepoRoot: true},
	".py":       {name: "ruff", cmd: "ruff", args: []string{"check", "--output-format", "concise"}},
	".pyi":      {name: "ruff", cmd: "ruff", args: []string{"check", "--output-format", "concise"}},
	".sh":       {name: "shellcheck", cmd: "shellcheck", args: []string{"--format", "gcc"}},
	".bash":     {name: "shellcheck", cmd: "shellcheck", args: []string{"--format", "gcc"}},
	".zsh":      {name: "zsh -n", cmd: "zsh", args: []string{"-n"}}, // syntax-only; shellcheck can't do zsh
	".yaml":     {name: "yamllint", cmd: "yamllint", args: []string{"-f", "parsable"}},
	".yml":      {name: "yamllint", cmd: "yamllint", args: []string{"-f", "parsable"}},
	".lua":      {name: "selene", cmd: "selene", args: []string{"--display-style", "quiet"}, configFile: "selene.toml"},
	".go":       {name: "golangci-lint", cmd: "golangci-lint", args: []string{"run"}, pkgScoped: true},
	// --strict is load-bearing: plain swiftlint exits 0 on a warning-only file, so
	// without it this rule never fires. configFile for the same reason selene
	// needs it. README (swift) has the rest, incl. why not swift-format.
	".swift": {name: "swiftlint", cmd: "swiftlint", args: []string{"lint", "--quiet", "--strict"}, configFile: ".swiftlint.yml"},
	// tofu fmt is a formatter, not a linter: -check exits non-zero when the file
	// isn't canonically formatted, -diff shows what would change. raw because its
	// output is a diff, not file:line issue lines.
	".tf":   {name: "tofu fmt", cmd: "tofu", args: []string{"fmt", "-check", "-diff", "-no-color"}, raw: true},
	".tofu": {name: "tofu fmt", cmd: "tofu", args: []string{"fmt", "-check", "-diff", "-no-color"}, raw: true},
}

// dockerfileLinter handles Dockerfiles, whose type is in the filename, not an
// extension (Dockerfile, api.Dockerfile).
var dockerfileLinter = linterSpec{name: "hadolint", cmd: "hadolint"}

// lintTimeout stays under the hook's 15s budget; a linter that overruns fails
// open rather than stalling the edit (golangci-lint on a big package is the
// realistic offender).
const lintTimeout = 10 * time.Second

// issueLine matches a "file:line" or "file:line:col" prefix — the shape every
// wired linter emits per problem. It's how banners/progress/summary lines
// (which have no :line:) get dropped so they don't masquerade as findings.
var issueLine = regexp.MustCompile(`:\d+[: ]`)

// adviseLint reports what the file's linter said. Advice, not a finding: the
// edit already landed and this never blocks, so a block envelope would render
// advisory output as a hook error. Fail-open at every step.
func adviseLint(ctx context.Context, ev *hook.Event) *hook.Advice {
	f, ok := ev.EditedFile()
	if !ok {
		return nil
	}
	path := filepath.Clean(f.Path)
	spec, ok := linterFor(path)
	if !ok {
		return nil // no linter registered for this file type
	}
	run, named := loadConfig().selected(spec.cmd)
	if !run {
		return nil // not in the user's list
	}
	if _, err := exec.LookPath(spec.cmd); err != nil {
		if named {
			// Fail closed: naming a linter is a request for it to run, so its
			// absence is a gap to report rather than a silent skip.
			return &hook.Advice{Text: hook.MissingBinary(RuleID, spec.cmd, installHint(spec.cmd)).Message}
		}
		return nil // not installed and not asked for — nothing to say
	}
	if _, err := os.Stat(path); err != nil {
		return nil // file gone (e.g. the edit deleted it) — nothing to lint
	}

	issues, ran := runLinter(ctx, spec, path)
	if !ran || len(issues) == 0 {
		return nil // clean, or couldn't run — hot path
	}

	// Stated as fact rather than as an instruction: text shaped like an
	// out-of-band command trips prompt-injection defences and gets shown to the
	// user instead of read as context.
	base := filepath.Base(path)
	var b strings.Builder
	b.WriteString(spec.name + " reported " + issueCount(len(issues)) + " in " + base + ":")
	for _, line := range issues {
		b.WriteString("\n  " + line)
	}
	return &hook.Advice{Text: b.String()}
}

func issueCount(n int) string {
	if n == 1 {
		return "1 issue"
	}
	return strconv.Itoa(n) + " issues"
}

// linterFor selects a linter by Dockerfile name first, then by extension. A
// .sh/.bash file that actually carries a zsh shebang is rerouted to the zsh
// linter: shellcheck can't parse zsh and aborts with SC1071 (e.g. ~/secrets.sh
// is #!/bin/zsh). Extension alone would misclassify it.
func linterFor(path string) (linterSpec, bool) {
	base := filepath.Base(path)
	if base == "Dockerfile" || strings.HasSuffix(base, ".Dockerfile") {
		return dockerfileLinter, true
	}
	spec, ok := lintersByExt[strings.ToLower(filepath.Ext(path))]
	if ok && spec.cmd == "shellcheck" && shebangIsZsh(path) {
		return lintersByExt[".zsh"], true
	}
	return spec, ok
}

// shebangIsZsh reports whether the file's first line is a zsh shebang
// (#!/bin/zsh, #!/usr/bin/env zsh, …). Unreadable/absent files report false so
// selection falls back to the extension default.
func shebangIsZsh(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 128)
	n, _ := f.Read(buf)
	first, _, _ := strings.Cut(string(buf[:n]), "\n")
	return strings.HasPrefix(first, "#!") && strings.Contains(first, "zsh")
}

// runLinter executes the linter on one file, in the file's own directory so
// module/project-aware tools (golangci-lint) resolve their context. ran is
// false when the command couldn't complete (timeout or spawn failure) — the
// caller treats that as fail-open. Exit 0 means clean; any non-zero exit means
// problems, whose output lines are returned (filtered unless spec.raw).
func runLinter(ctx context.Context, spec linterSpec, path string) (issues []string, ran bool) {
	// Only when the caller set no deadline. The framework already bounds a rule
	// by its configured timeout, and clamping again here made `timeout = 60` on
	// a monorepo do nothing: golangci-lint still died at ten seconds.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, lintTimeout)
		defer cancel()
	}

	args := append([]string{}, spec.args...)
	if spec.dynArgs != nil {
		args = append(args, spec.dynArgs()...)
	}
	// pkgScoped tools lint the containing dir (package); others lint the one file.
	if spec.pkgScoped {
		args = append(args, ".")
	} else {
		args = append(args, path)
	}

	cmd := exec.CommandContext(ctx, spec.cmd, args...)
	cmd.Dir = filepath.Dir(path)
	if spec.configFile != "" {
		if dir := findConfigDir(cmd.Dir, spec.configFile); dir != "" {
			cmd.Dir = dir
		}
	}
	if spec.fromRepoRoot {
		if root := gitRepoRoot(ctx, cmd.Dir); root != "" {
			cmd.Dir = root
		}
	}
	out, err := cmd.CombinedOutput()

	if ctx.Err() != nil {
		return nil, false // timed out
	}
	if err == nil {
		return nil, true // exit 0 — clean
	}
	if _, isExit := err.(*exec.ExitError); !isExit {
		return nil, false // couldn't spawn/run — fail-open
	}
	issues = filterLines(string(out), spec.raw)
	if spec.pkgScoped {
		issues = keepFile(issues, filepath.Base(path)) // re-narrow to the edited file
	}
	return issues, true
}

// keepFile drops findings that don't refer to base. Package-scoped linters
// analyze the whole package so cross-file symbols resolve; this re-narrows the
// output to the file that was just edited, matching the single-file semantics
// every other linter has. A finding's path is the text before its first ":".
func keepFile(lines []string, base string) []string {
	var kept []string
	for _, ln := range lines {
		p, _, ok := strings.Cut(ln, ":")
		if ok && filepath.Base(strings.TrimSpace(p)) == base {
			kept = append(kept, ln)
		}
	}
	return kept
}

// filterLines splits linter output into per-problem strings, dropping blanks.
// For normal linters it keeps only lines with a :line[:col] locator so banners
// and summaries don't masquerade as findings; raw tools (tofu fmt's diff) keep
// every non-empty line.
func filterLines(out string, raw bool) []string {
	var lines []string
	for ln := range strings.SplitSeq(out, "\n") {
		s := strings.TrimSpace(ln)
		if s == "" {
			continue
		}
		if raw || issueLine.MatchString(s) {
			lines = append(lines, s)
		}
	}
	return lines
}

// findConfigDir walks up from dir and returns the nearest ancestor directory
// (dir included) that directly contains a file named name, or "" if none is
// found before the filesystem root. Lets a linter run from the directory
// holding its config even when the tool can't discover parent configs itself.
func findConfigDir(dir, name string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// gitRepoRoot returns the git working-tree root containing dir, or "" if dir
// isn't inside a git repo (or git is unavailable). Used to run repo-aware
// linters from the root so their config discovery reaches a repo-local config.
func gitRepoRoot(ctx context.Context, dir string) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// markdownlintConfigArgs points markdownlint-cli2 at the home-dir global config
// with --config. Needed because cli2's own config discovery stops at the git
// repo root, so ~/.markdownlint-cli2.jsonc never reaches files inside a repo on
// its own. Returns nil when the file is absent (fresh machine pre-dotfiles),
// so markdownlint just runs with its defaults instead of erroring.
func markdownlintConfigArgs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	cfg := filepath.Join(home, ".markdownlint-cli2.jsonc")
	if _, err := os.Stat(cfg); err != nil {
		return nil
	}
	return []string{"--config", cfg}
}
