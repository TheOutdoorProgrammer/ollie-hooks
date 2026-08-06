// Command ollie-hooks is a Claude Code hook gate. Claude Code invokes it once
// per hook event with the event JSON on stdin; it runs the registered rules and
// prints the response envelope on stdout.
//
// Fails open by design: any internal error logs to stderr and exits 0, so a bug
// here can never wedge a session.
package main

import (
	"fmt"
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

	// Subcommands: everything else is hook mode (a JSON event on stdin).
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "wiring":
			hook.PrintWiring()
			return
		case "version", "--version", "-v":
			fmt.Printf("ollie-hooks %s (%s)\n", version, commit)
			return
		}
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
