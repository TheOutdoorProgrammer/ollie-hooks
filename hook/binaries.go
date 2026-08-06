package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Tool is an external binary a rule needs. Install is shown verbatim when it is
// absent, so it should be a command the user can paste.
type Binary struct {
	Bin     string
	Install string
}

// binaryCacheTTL bounds how long a positive lookup is trusted. Only successes are
// cached: caching a miss would mean installing the tool had no effect until the
// entry expired, which is precisely when someone is waiting for it to work.
const binaryCacheTTL = 24 * time.Hour

type binaryCache struct {
	Found   map[string]bool `json:"found"`
	Written time.Time       `json:"written"`
}

func binaryCachePath() string {
	return filepath.Join(stateDir(), "binary-cache.json")
}

// MissingBinaries returns required tools that are absent. Resolving a binary walks
// PATH, wasted work on the hot path where everything is present, so a positive
// result is remembered for a day.
func MissingBinaries(tools []Binary) []Binary {
	if len(tools) == 0 {
		return nil
	}
	cache := loadBinaryCache()
	var missing []Binary
	dirty := false
	for _, t := range tools {
		if cache.Found[t.Bin] {
			continue
		}
		if Installed(t.Bin) {
			cache.Found[t.Bin] = true
			dirty = true
			continue
		}
		missing = append(missing, t)
	}
	if dirty {
		saveBinaryCache(cache)
	}
	return missing
}

func loadBinaryCache() binaryCache {
	fresh := binaryCache{Found: map[string]bool{}, Written: time.Now()}
	data, err := os.ReadFile(binaryCachePath())
	if err != nil {
		return fresh
	}
	var c binaryCache
	if err := json.Unmarshal(data, &c); err != nil || c.Found == nil {
		return fresh
	}
	if time.Since(c.Written) > binaryCacheTTL {
		return fresh
	}
	return c
}

// saveBinaryCache writes through a temp file so a concurrent hook never reads a
// half-written cache — hooks run in parallel across tool calls.
func saveBinaryCache(c binaryCache) {
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "binary-cache-*.json")
	if err != nil {
		return
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return
	}
	if err := os.Rename(name, binaryCachePath()); err != nil {
		os.Remove(name)
	}
}

// missingBinaryFindings turns absent tools into the standard fail-closed report.
func missingBinaryFindings(rule string, missing []Binary) []Finding {
	out := make([]Finding, 0, len(missing))
	for _, t := range missing {
		out = append(out, MissingBinary(rule, t.Bin, t.Install))
	}
	return out
}
