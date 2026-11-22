package fakelsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRenameCoversOpenAndClosedFilesOfTheFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("x := total\ntotals := 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := "file://" + filepath.Join(dir, "a.go")
	s := &server{docs: map[string]string{a: "var total = 1\n"}}
	params := json.RawMessage(`{"textDocument":{"uri":"` + a + `"},"position":{"line":0,"character":5},"newName":"sum"}`)
	edit := s.refactor("textDocument/rename", params).(map[string]any)["changes"].(map[string][]map[string]any)
	if len(edit[a]) != 1 || len(edit["file://"+filepath.Join(dir, "b.go")]) != 1 {
		t.Fatalf("rename edits = %v, want one in a.go and one in b.go (not totals)", edit)
	}
	refs := s.refactor("textDocument/references", params).([]map[string]any)
	if len(refs) != 2 {
		t.Fatalf("got %d references, want 2", len(refs))
	}
	if s.refactor("textDocument/rename", json.RawMessage(`{"textDocument":{"uri":"`+a+`"},"position":{"line":1,"character":0}}`)) != nil {
		t.Fatal("an empty line found a name")
	}
}
