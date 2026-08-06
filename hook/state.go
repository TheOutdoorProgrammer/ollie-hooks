package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// State is a rule's JSON scratch store. It lives in the state dir, never the
// config dir: this is regenerable data nobody edits by hand.
type State struct {
	rule string
	key  string
}

// RuleState opens the store for one rule. The rule ID namespaces the files, so
// two rules cannot collide.
func RuleState(ruleID string) *State {
	return &State{rule: ruleID}
}

// Scoped narrows the store to one instance of whatever the rule is tracking —
// a session, a message, a file. Parts are hashed, so they may contain anything.
func (s *State) Scoped(parts ...string) *State {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return &State{rule: s.rule, key: hex.EncodeToString(sum[:8])}
}

func (s *State) path() string {
	name := s.rule
	if s.key != "" {
		name += "-" + s.key
	}
	return filepath.Join(stateDir(), name+".json")
}

// Load decodes the stored value into v and reports whether anything was read.
func (s *State) Load(v any) bool {
	data, err := os.ReadFile(s.path())
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
}

// Save writes v, creating the state directory on demand.
func (s *State) Save(v any) error {
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), data, 0o600)
}

// Clear removes this entry and sweeps orphans from scopes that never finished.
// TempDir used to give us that cleanup for free; the state dir does not.
func (s *State) Clear() {
	_ = os.Remove(s.path())
	sweepStale(stateDir())
}
