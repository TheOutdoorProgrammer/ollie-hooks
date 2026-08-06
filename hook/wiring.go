package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// PrintWiring prints the settings.json hook entries from the registry so the
// manual settings can't drift. Hooks are keyed by event name, so this is one
// entry per event with rules — not one entry overall.
func PrintWiring() {
	bin := binaryPath()
	fmt.Println("# ~/.claude/settings.json hook entries (append to the existing arrays):")
	fmt.Println("#")
	fmt.Println("# The timeout is deliberately long. Every rule is already bounded")
	fmt.Println("# in-process, and a process Claude Code kills returns NOTHING —")
	fmt.Println("# abandoning one slow rule still returns what the others found.")
	fmt.Println()
	// Derived from the registry, not a fixed list: an event with no rules must
	// not be printed, or the settings grow an entry that does nothing.
	for _, event := range registeredEvents() {
		// Exec form ("args" present): the binary is spawned directly, so no
		// shell tokenisation of the path and no profile output corrupting stdout.
		fmt.Printf("%q: [{ \"matcher\": %q, \"hooks\": [{ \"type\": \"command\", \"command\": %q, \"args\": [], \"timeout\": %d }] }]\n",
			event, eventMatcher(event), bin, eventTimeout(event))
	}
}

// registeredEvents lists the hook events that actually have rules, in
// registration order so the printed wiring is stable between runs.
func registeredEvents() []EventName {
	var events []EventName
	for _, r := range registry {
		for _, e := range r.Events {
			if !slices.Contains(events, e) {
				events = append(events, e)
			}
		}
	}
	return events
}

// eventCeiling is the largest Timeout among rules on this event, clamped to the
// platform's own cap — a settings timeout above it RAISES the real budget, so

// binaryPath resolves this executable's install path for the wiring snippet,
// falling back to ~/bin/ollie-hooks (where make install puts it).
func binaryPath() string {
	if p, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			return resolved
		}
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "bin", "ollie-hooks")
	}
	return "ollie-hooks"
}
