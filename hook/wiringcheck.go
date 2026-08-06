package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// WiringDrift is the set of events the registry needs but settings.json does
// not run ollie-hooks on. A missing event fires no rules, so a rule added on a
// fresh event stays dead until settings are rewired.
type WiringDrift struct {
	SettingsPath  string
	SettingsFound bool
	// Missing are events carrying rules that settings does not run ollie-hooks
	// on, in registration order.
	Missing []EventName
}

// Clean reports no drift: every event with rules is wired.
func (d WiringDrift) Clean() bool { return len(d.Missing) == 0 }

// CheckWiring diffs the registry's needed events against settings.json and
// reports which are unwired. A settings file that is absent or unreadable
// counts as nothing wired, so every needed event is reported.
func CheckWiring() WiringDrift {
	path := claudeSettingsPath()
	wired, found := wiredEvents(path)
	d := WiringDrift{SettingsPath: path, SettingsFound: found}
	for _, e := range registeredEvents() {
		if !wired[e] {
			d.Missing = append(d.Missing, e)
		}
	}
	return d
}

// claudeSettingsPath is where Claude Code keeps its hook wiring.
// CLAUDE_CONFIG_DIR overrides the default so a test can point it somewhere
// disposable, the same way the real harness relocates its config.
func claudeSettingsPath() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "settings.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// wiredEvents reads settings.json and returns the events whose hooks invoke
// ollie-hooks. found is false when the file is missing or unparseable, which
// callers treat as nothing wired rather than an error.
func wiredEvents(path string) (map[EventName]bool, bool) {
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var s struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, false
	}
	wired := map[EventName]bool{}
	for event, entries := range s.Hooks {
		for _, entry := range entries {
			for _, h := range entry.Hooks {
				if invokesOllieHooks(h.Command) {
					wired[EventName(event)] = true
				}
			}
		}
	}
	return wired, true
}

// invokesOllieHooks reports whether a settings command runs this binary. The
// command may carry arguments, so the binary name is matched as a whole token,
// not a substring that "ollie-hooks-helper" would also satisfy.
func invokesOllieHooks(command string) bool {
	for _, tok := range strings.Fields(command) {
		if filepath.Base(tok) == appName {
			return true
		}
	}
	return false
}

// WriteWiringCheck renders the wiring diff and reports whether it is clean, so
// the CLI can exit non-zero on drift. Reuses the same diff doctor reports.
func WriteWiringCheck(w io.Writer) bool {
	d := CheckWiring()
	if !d.SettingsFound {
		_, _ = fmt.Fprintln(w, Paint("settings.json not found at "+d.SettingsPath, DraculaRed))
		_, _ = fmt.Fprintln(w, "Nothing is wired. Run `ollie-hooks wiring` and add the entries.")
	}
	if d.Clean() {
		_, _ = fmt.Fprintln(w, Paint("wiring OK: every event with rules is wired.", DraculaGreen))
		return true
	}
	if d.SettingsFound {
		_, _ = fmt.Fprintln(w, Paint("wiring drift: events with rules that are NOT wired:", DraculaRed))
	}
	for _, e := range d.Missing {
		_, _ = fmt.Fprintf(w, "  %s\n", Paint(string(e), DraculaOrange))
	}
	_, _ = fmt.Fprintln(w, "Fix: run `ollie-hooks wiring` and append the missing entries to settings.json.")
	return false
}
