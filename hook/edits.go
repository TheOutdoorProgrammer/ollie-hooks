package hook

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// TextEdit is one before/after pair from a file-editing tool. A Write has no
// before, so Before is empty and After is the whole file.
type TextEdit struct {
	Before string
	After  string
}

// EditedFile is what a Write, Edit or MultiEdit did to one file. Scoping a
// PostToolUse rule to what a change introduced, rather than the whole file, is
// what keeps it from screaming about pre-existing code on every edit.
type EditedFile struct {
	Path  string
	Edits []TextEdit
}

// Added is the introduced text alone, for rules with no use for the old.
func (f EditedFile) Added() []string {
	out := make([]string, 0, len(f.Edits))
	for _, e := range f.Edits {
		out = append(out, e.After)
	}
	return out
}

// Ext is the file's lowercased extension, including the dot. Lowercased
// because a rule keyed on ".md" must still fire on README.MD.
func (f EditedFile) Ext() string {
	return strings.ToLower(filepath.Ext(f.Path))
}

// EditedFile decodes a file-editing tool call. ok is false for any other tool,
// or when the input carries no readable path — a shape we cannot read is not
// something to guess at.
func (e *Event) EditedFile() (EditedFile, bool) {
	var in struct {
		FilePath string `json:"file_path"`
		// NotebookEdit carries its path under a different key, and a rule that
		// only reads file_path lets every notebook edit through unexamined.
		NotebookPath string `json:"notebook_path"`
		Content      string `json:"content"`
		OldString    string `json:"old_string"`
		NewString    string `json:"new_string"`
		NewSource    string `json:"new_source"`
		OldSource    string `json:"old_source"`
		Edits        []struct {
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		} `json:"edits"`
	}
	if err := json.Unmarshal(e.ToolInput, &in); err != nil {
		return EditedFile{}, false
	}
	path := in.FilePath
	if path == "" {
		path = in.NotebookPath
	}
	if path == "" {
		return EditedFile{}, false
	}

	f := EditedFile{Path: absolutise(path, e.CWD)}
	switch e.ToolName {
	case "Write":
		f.Edits = []TextEdit{{After: in.Content}}
	case "Edit":
		f.Edits = []TextEdit{{Before: in.OldString, After: in.NewString}}
	case "NotebookEdit":
		f.Edits = []TextEdit{{Before: in.OldSource, After: in.NewSource}}
	case "MultiEdit":
		for _, ed := range in.Edits {
			f.Edits = append(f.Edits, TextEdit{Before: ed.OldString, After: ed.NewString})
		}
	default:
		return EditedFile{}, false
	}
	return f, true
}

// absolutise resolves a relative tool path against the event's working
// directory, so a path-matching rule sees the same string however the tool
// call happened to be phrased.
func absolutise(path, cwd string) string {
	if cwd == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
}
