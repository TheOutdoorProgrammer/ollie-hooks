package nocodegen

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// checkNoCodegen blocks writes to a tree marked off-limits to AI code. It
// guards the one surface a hook catches deterministically — a written file —
// and claims nothing about pasted code, which it cannot see.
func checkNoCodegen(ctx context.Context, ev *hook.Event) []hook.Finding {
	cfg := loadConfig()
	if len(cfg.Paths) == 0 {
		return nil
	}

	path := targetPath(ev)
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(ev.CWD, path)
	}
	target := normalize(path)

	for _, p := range cfg.Paths {
		if p.Match == "" || !strings.Contains(target, normalize(p.Match)) {
			continue
		}
		return []hook.Finding{{Message: guidance(p)}}
	}
	return nil
}

// targetPath is the file a write-shaped tool is aimed at. NotebookEdit names it
// notebook_path, and missing that field would silently exempt every notebook.
func targetPath(ev *hook.Event) string {
	var in struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	}
	if err := json.Unmarshal(ev.ToolInput, &in); err != nil {
		return "" // fail-open: a shape we can't read is not a violation
	}
	if in.FilePath != "" {
		return in.FilePath
	}
	return in.NotebookPath
}

// normalize makes matching separator-agnostic. On Windows tool_input paths
// arrive with backslashes, so a forward-slash pattern would silently never
// match and the write would proceed as if the rule had nothing to say.
func normalize(p string) string {
	return strings.ReplaceAll(filepath.Clean(p), `\`, "/")
}

func guidance(p protectedPath) string {
	reason := strings.TrimSpace(p.Reason)
	if reason == "" {
		reason = "This tree is configured as off-limits to AI-authored code."
	}
	return `
		Blocked: ` + reason + `
		You may not create or edit files here, and you must not hand over
		copy-pasteable fix code in chat either — that is the same thing with an
		extra step. Assisting IS allowed: read the code, investigate, cite
		specific files and line numbers, explain why something is wrong and
		where a fix belongs, and offer high-level approaches. Deliver findings
		as prose the user acts on, never as a patch.`
}
