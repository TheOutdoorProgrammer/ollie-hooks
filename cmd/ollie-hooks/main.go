// Command ollie-hooks is a Claude Code hook gate. Claude Code invokes it once
// per hook event with the event JSON on stdin; it runs the registered rules and
// prints the response envelope on stdout.
//
// Fails open by design: any internal error logs to stderr and exits 0, so a bug
// here can never wedge a session.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules"
)

// Set by the release build via -ldflags. A source build reports "dev", which is
// the honest answer to "what version is this" in a bug report.
var (
	version = "dev"
	commit  = "none"
)

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `ollie-hooks — a rule engine for Claude Code hooks.

Usage:
  ollie-hooks rules            What rules exist, and what your config does with them.
  ollie-hooks config example   A commented config covering every rule.
  ollie-hooks wiring           The settings.json entries that wire this up.
  ollie-hooks wiring --check   Diff the wiring against settings.json; fail on drift.
  ollie-hooks doctor           Diagnose your setup; exit non-zero if anything is off.
  ollie-hooks trust [dir]      Apply this repo's .ollie-hooks.toml config.
  ollie-hooks untrust [dir]    Stop applying this repo's project config.
  ollie-hooks docs             The markdown rule reference.
  ollie-hooks version          Print the version.
  ollie-hooks help             Print this.

With no arguments it reads one hook event as JSON on stdin and writes the
response envelope on stdout, which is how Claude Code invokes it.

Rules ship disabled. Enable them in:
  ~/.config/ollie-hooks/config.toml

Set OLLIE_HOOKS_DEBUG=1 to trace which rules ran, and why the others did not.
Docs: https://github.com/TheOutdoorProgrammer/ollie-hooks
`)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "ollie-hooks: %v\n", r)
			os.Exit(0) // fail open
		}
	}()

	rules.RegisterAll()
	// A broken plugin entry is reported and skipped, never fatal: someone else's
	// config typo must not take away the rules that do work.
	for _, err := range hook.RegisterCustomRules() {
		fmt.Fprintf(os.Stderr, "ollie-hooks: %v\n", err)
	}

	// Claude Code invokes this with no arguments, so anything here came from a
	// person. An unrecognised word must not fall through to hook mode, which
	// would block reading an event off a terminal that will never send one.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "wiring":
			if len(os.Args) > 2 && os.Args[2] == "--check" {
				if !hook.WriteWiringCheck(os.Stdout) {
					os.Exit(1)
				}
				return
			}
			hook.PrintWiring()
		case "doctor":
			if !hook.WriteDoctor(os.Stdout) {
				os.Exit(1)
			}
		case "rules":
			hook.WriteRuleList(os.Stdout)
		case "docs":
			hook.WriteRuleDocs(os.Stdout)
		case "config":
			if len(os.Args) > 2 && os.Args[2] == "example" {
				hook.WriteConfigExample(os.Stdout)
				return
			}
			fmt.Fprintln(os.Stderr, "ollie-hooks: usage: ollie-hooks config example")
			os.Exit(2)
		case "trust":
			trustCmd(os.Args[2:], true)
		case "untrust":
			trustCmd(os.Args[2:], false)
		case "version", "--version", "-v":
			fmt.Printf("ollie-hooks %s (%s)\n", version, commit)
		case "help", "--help", "-h":
			usage(os.Stdout)
		default:
			fmt.Fprintf(os.Stderr, "ollie-hooks: unknown command %q\n\n", os.Args[1])
			usage(os.Stderr)
			os.Exit(2)
		}
		return
	}

	// DecodeEvent, not a plain Decode: it keeps the raw payload so per-event
	// accessors can reach fields the flat struct does not carry.
	ev, err := hook.DecodeEvent(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ollie-hooks: decoding event: %v\n", err)
		return
	}

	out, err := hook.Decide(ev)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ollie-hooks: %v\n", err)
		return
	}
	if out != "" {
		fmt.Println(out)
	}
}

// trustCmd trusts or untrusts a project so its .ollie-hooks.toml is applied or
// ignored. With no argument it acts on the project found from the working dir.
func trustCmd(args []string, trust bool) {
	dir := ""
	if len(args) > 0 {
		dir = args[0]
	} else {
		dir = hook.ProjectConfigDir()
	}
	if dir == "" {
		fmt.Fprintln(os.Stderr, "ollie-hooks: no .ollie-hooks.toml found here; pass a directory")
		os.Exit(2)
	}
	verb, ok := "trusted", hook.Trust(dir)
	if !trust {
		verb, ok = "untrusted", hook.Untrust(dir)
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "ollie-hooks: could not update the trust list")
		os.Exit(1)
	}
	fmt.Printf("%s %s\n", verb, dir)
}
