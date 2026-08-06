package hook

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const appName = "ollie-hooks"

// staleStateAge bounds how long an orphaned state file survives. Anything this
// old belongs to a message that never reached Final: interrupted, or crashed.
const staleStateAge = 24 * time.Hour

// configDir is where user config lives. Empty when there is no resolvable home,
// which callers treat as "no config" rather than an error.
func configDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, appName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", appName)
}

// configPath resolves one config file by name, or "" when there is no home.
func configPath(name string) string {
	dir := configDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, name)
}

// stateDir holds regenerable scratch, deliberately not the config dir.
// CLAUDE_PLUGIN_DATA wins: Claude Code guarantees it survives plugin updates.
func stateDir() string {
	if d := os.Getenv("CLAUDE_PLUGIN_DATA"); d != "" {
		return filepath.Join(d, "state")
	}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, appName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), appName)
	}
	return filepath.Join(home, ".local", "state", appName)
}

// sweepStale deletes state files older than staleStateAge. Call it on a
// once-per-message path, never per delta — MessageDisplay is the hot event.
func sweepStale(dir, rule string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-staleStateAge)
	for _, e := range entries {
		// Only this rule's own files. Sweeping the whole directory made one
		// rule's cleanup delete another's state, and the binary cache with it.
		if !ownedBy(e.Name(), rule) {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// ownedBy reports whether a state file belongs to a rule. Files are named
// <rule>.json or <rule>-<key>.json, so a bare prefix test would also match an
// unrelated rule whose name merely starts the same way.
func ownedBy(name, rule string) bool {
	rest, ok := strings.CutPrefix(name, rule)
	return ok && (rest == ".json" || strings.HasPrefix(rest, "-"))
}
