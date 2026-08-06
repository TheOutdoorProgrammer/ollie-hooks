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
	IgnorePaths []string `toml:"ignore_paths" doc:"skip any path containing one of these"`
	// Markdown by default because that is where renderers rejoin lines.
	Extensions []string `toml:"extensions" doc:"file extensions this applies to"`
	// A reflowed file can produce hundreds, and a wall of them buries the point.
	MaxReported int `toml:"max_reported" doc:"stop listing wrapped lines after this many"`
}

func defaultConfig() config {
	return config{
		Extensions:  []string{".md", ".markdown"},
		MaxReported: maxProseWraps,
	}
}

func loadConfig() config { return hook.Config(RuleID, defaultConfig()) }

// Validate refills the extension list and report cap when a section blanks it.
func (c *config) Validate() {
	d := defaultConfig()
	if len(c.Extensions) == 0 {
		c.Extensions = d.Extensions
	}
	if c.MaxReported <= 0 {
		c.MaxReported = d.MaxReported
	}
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
