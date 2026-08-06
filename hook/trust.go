package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// trustFile lists project directories whose .ollie-hooks.toml the user vouched
// for. Until a directory is listed here, its project config does nothing.
const trustFile = "trust.toml"

type trustList struct {
	Trusted []string `toml:"trusted"`
}

// IsTrusted reports whether a project directory's config may be applied. The
// path is cleaned so "/a/b" and "/a/b/" resolve to one entry.
func IsTrusted(dir string) bool {
	dir = cleanDir(dir)
	if dir == "" {
		return false
	}
	for _, t := range readTrust().Trusted {
		if cleanDir(t) == dir {
			return true
		}
	}
	return false
}

// Trust records a project directory so its config applies. Returns false when
// there is no writable config dir.
func Trust(dir string) bool {
	dir = cleanDir(dir)
	if dir == "" {
		return false
	}
	list := readTrust()
	for _, t := range list.Trusted {
		if cleanDir(t) == dir {
			return true
		}
	}
	list.Trusted = append(list.Trusted, dir)
	return writeTrust(list)
}

// Untrust drops a project directory so its config stops applying.
func Untrust(dir string) bool {
	dir = cleanDir(dir)
	list := readTrust()
	kept := make([]string, 0, len(list.Trusted))
	for _, t := range list.Trusted {
		if cleanDir(t) != dir {
			kept = append(kept, t)
		}
	}
	list.Trusted = kept
	return writeTrust(list)
}

// TrustedProjects returns the trusted directories, cleaned and sorted, for
// reporting by doctor.
func TrustedProjects() []string {
	raw := readTrust().Trusted
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		out = append(out, cleanDir(t))
	}
	sort.Strings(out)
	return out
}

// cleanDir resolves a directory to an absolute, cleaned path for comparison.
func cleanDir(dir string) string {
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return filepath.Clean(dir)
}

func readTrust() trustList {
	path := configPath(trustFile)
	if path == "" {
		return trustList{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return trustList{}
	}
	var l trustList
	if _, err := toml.Decode(string(data), &l); err != nil {
		return trustList{}
	}
	return l
}

// writeTrust persists the list through a temp file and rename, so a concurrent
// hook never reads a half-written file.
func writeTrust(l trustList) bool {
	dir := configDir()
	if dir == "" {
		return false
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(l); err != nil {
		return false
	}
	tmp, err := os.CreateTemp(dir, trustFile+".*")
	if err != nil {
		return false
	}
	name := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return false
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return false
	}
	if err := os.Rename(name, configPath(trustFile)); err != nil {
		_ = os.Remove(name)
		return false
	}
	return true
}
