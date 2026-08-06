package hook

import (
	"encoding/json"
	"testing"
)

func editEvent(t *testing.T, tool string, input map[string]any) *Event {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return &Event{HookEventName: PostToolUse, ToolName: tool, ToolInput: raw}
}

// A Write has no before, so the whole file counts as introduced.
func TestEditedFileWrite(t *testing.T) {
	ev := editEvent(t, "Write", map[string]any{"file_path": "/a/b.md", "content": "hello"})
	f, ok := ev.EditedFile()
	if !ok || f.Path != "/a/b.md" || len(f.Edits) != 1 {
		t.Fatalf("Write = %+v ok=%v", f, ok)
	}
	if f.Edits[0].Before != "" || f.Edits[0].After != "hello" {
		t.Errorf("a Write should be one edit with no before, got %+v", f.Edits[0])
	}
	if f.Ext() != ".md" {
		t.Errorf("Ext = %q", f.Ext())
	}
}

func TestEditedFileEditKeepsBothSides(t *testing.T) {
	ev := editEvent(t, "Edit", map[string]any{
		"file_path": "/a/b.go", "old_string": "was", "new_string": "is",
	})
	f, ok := ev.EditedFile()
	if !ok || len(f.Edits) != 1 {
		t.Fatalf("Edit = %+v ok=%v", f, ok)
	}
	if f.Edits[0].Before != "was" || f.Edits[0].After != "is" {
		t.Errorf("both sides must survive, got %+v", f.Edits[0])
	}
	if got := f.Added(); len(got) != 1 || got[0] != "is" {
		t.Errorf("Added should be the new text only, got %v", got)
	}
}

func TestEditedFileMultiEditKeepsEveryPair(t *testing.T) {
	ev := editEvent(t, "MultiEdit", map[string]any{
		"file_path": "/a/b.go",
		"edits": []map[string]string{
			{"old_string": "a", "new_string": "A"},
			{"old_string": "b", "new_string": "B"},
		},
	})
	f, ok := ev.EditedFile()
	if !ok || len(f.Edits) != 2 {
		t.Fatalf("MultiEdit = %+v ok=%v", f, ok)
	}
	if f.Edits[1].Before != "b" || f.Edits[1].After != "B" {
		t.Errorf("second edit lost, got %+v", f.Edits[1])
	}
}

// A shape we cannot read is not something to guess at.
func TestEditedFileDeclinesOtherTools(t *testing.T) {
	for _, tool := range []string{"Bash", "Read", "Grep"} {
		ev := editEvent(t, tool, map[string]any{"file_path": "/a/b.go"})
		if _, ok := ev.EditedFile(); ok {
			t.Errorf("%s is not a file-editing tool", tool)
		}
	}
	ev := &Event{HookEventName: PostToolUse, ToolName: "Write", ToolInput: []byte("not json")}
	if _, ok := ev.EditedFile(); ok {
		t.Error("unreadable input must decline")
	}
	noPath := editEvent(t, "Write", map[string]any{"content": "x"})
	if _, ok := noPath.EditedFile(); ok {
		t.Error("a call with no path must decline")
	}
}
