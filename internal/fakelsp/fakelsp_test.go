package fakelsp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormattingTrimsTrailingWhiteSpace(t *testing.T) {
	s := &server{docs: map[string]string{"file:///a.go": "one  \ntwo\nthree\t\n"}}
	edits := s.formatting(json.RawMessage(`{"textDocument":{"uri":"file:///a.go"}}`))
	if len(edits) != 2 {
		t.Fatalf("got %d edits, want 2: %v", len(edits), edits)
	}
	start := edits[1]["range"].(map[string]any)["start"].(map[string]int)
	if start["line"] != 2 || start["character"] != 5 {
		t.Fatalf("second edit starts at %v, want line 2 character 5", start)
	}
}

func TestFormattingOutOfRangeTrigger(t *testing.T) {
	s := &server{docs: map[string]string{"file:///a.go": "OUT_OF_RANGE\n"}}
	edits := s.formatting(json.RawMessage(`{"textDocument":{"uri":"file:///a.go"}}`))
	if len(edits) != 1 {
		t.Fatalf("got %d edits, want 1", len(edits))
	}
	end := edits[0]["range"].(map[string]any)["end"].(map[string]int)
	if end["character"] != 999 {
		t.Fatalf("edit ends at %v, want character 999", end)
	}
}

func TestFormattingRequestIsAnswered(t *testing.T) {
	var out bytes.Buffer
	s := &server{docs: map[string]string{"file:///a.go": "one  \n"}, out: &out}
	s.handle(msg{ID: json.RawMessage(`7`), Method: "textDocument/formatting", Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.go"}}`)})
	if !strings.Contains(out.String(), `"id":7`) || !strings.Contains(out.String(), `"newText":""`) {
		t.Fatalf("no answer with an edit: %q", out.String())
	}
}

func TestSymbolsNestByIndent(t *testing.T) {
	got := symbols("package main\n\ntype Server struct {\n\tfunc handleLogin() {\n\t\tx := 1\n\t}\n}\n\nfunc other() {}\n")
	if len(got) != 2 || got[0].Name != "Server" || got[1].Name != "other" {
		t.Fatalf("top level: %+v", got)
	}
	if len(got[0].Children) != 1 || got[0].Children[0].Name != "handleLogin" {
		t.Fatalf("children of Server: %+v", got[0].Children)
	}
	end := got[0].Children[0].Range["end"].(map[string]int)
	if end["line"] != 5 {
		t.Fatalf("handleLogin ends at %v, want line 5", end)
	}
	if outer := got[0].Range["end"].(map[string]int); outer["line"] != 6 {
		t.Fatalf("Server ends at %v, want line 6", outer)
	}
	if sel := got[0].Children[0].SelectionRange["start"].(map[string]int); sel["line"] != 3 || sel["character"] != 6 {
		t.Fatalf("handleLogin name starts at %v", sel)
	}
}

func TestInitDelay(t *testing.T) {
	dir := t.TempDir()
	params := json.RawMessage(`{"rootUri":"file://` + dir + `"}`)
	if d := initDelay(params); d != 0 {
		t.Fatalf("no marker file, delay %v", d)
	}
	marker := filepath.Join(dir, ".fakelsp-slow-init")
	if err := os.WriteFile(marker, []byte("1500ms\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if d := initDelay(params); d != 1500*time.Millisecond {
		t.Fatalf("delay %v, want 1.5s", d)
	}
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if d := initDelay(params); d != 4*time.Second {
		t.Fatalf("empty marker, delay %v, want 4s", d)
	}
}

func TestDocumentSymbolIsAnswered(t *testing.T) {
	var out bytes.Buffer
	s := &server{docs: map[string]string{"file:///a.go": "func one() {}\n"}, out: &out}
	s.handle(msg{ID: json.RawMessage(`9`), Method: "textDocument/documentSymbol", Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.go"}}`)})
	if !strings.Contains(out.String(), `"name":"one"`) {
		t.Fatalf("no symbol: %q", out.String())
	}
}
