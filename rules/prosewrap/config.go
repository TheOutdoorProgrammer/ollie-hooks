package prosewrap

import (
	"path/filepath"
	"strings"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// config tunes the prose gate. One-sentence-per-line is a defensible position,
// not a correctness rule: plenty of people hard-wrap at 80 and are happy. So
// everything about it is the user's to set.
type config struct {
	// IgnorePaths are substrings matched against the file path, for trees that
	// are already hard-wrapped and would otherwise flag on every edit.
	IgnorePaths []string `toml:"ignore_paths"`
	// Extensions this applies to. Markdown by default because that is where
	// renderers rejoin lines, but .mdx and .txt are reasonable additions.
	Extensions []string `toml:"extensions"`
	// MaxReported bounds how many wraps one finding lists. A reflowed file can
	// produce hundreds, and a wall of them buries the instruction.
	MaxReported int `toml:"max_reported"`
}

func defaultConfig() config {
	return config{
		Extensions:  []string{".md", ".markdown"},
		MaxReported: maxProseWraps,
	}
}

func loadConfig() config {
	cfg := hook.Config(RuleID, defaultConfig())
	if len(cfg.Extensions) == 0 {
		cfg.Extensions = defaultConfig().Extensions
	}
	if cfg.MaxReported <= 0 {
		cfg.MaxReported = defaultConfig().MaxReported
	}
	return cfg
}

// applies reports whether this file is prose the rule should read.
func (c config) applies(path string) bool {
	for _, ig := range c.IgnorePaths {
		if ig != "" && strings.Contains(path, ig) {
			return false
		}
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range c.Extensions {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}
