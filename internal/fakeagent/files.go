package fakeagent

import (
	"encoding/json"
	"os"
	"strings"
)

// fileTurn answers "readfile <path>" and "readlimit <path>" (reads through the
// client) and "WRITEFILE <abs path> <text>" (writes the file itself, as an agent
// that edits files does).
func (a *agent) fileTurn(sid, text string) {
	verb, rest, _ := strings.Cut(text, " ")
	switch verb {
	case "readfile":
		res := a.call("fs/read_text_file", map[string]any{"sessionId": sid, "path": rest})
		if len(res.Error) > 0 {
			a.chunk(sid, "error:"+string(res.Error))
		} else {
			a.chunk(sid, "read:"+string(res.Result))
		}
	case "readlimit":
		res := a.call("fs/read_text_file", map[string]any{"sessionId": sid, "path": rest, "line": 2, "limit": json.Number("9223372036854775807")})
		a.chunk(sid, "read:"+string(res.Result)+string(res.Error))
	case "WRITEFILE":
		path, body, _ := strings.Cut(rest, " ")
		if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
			a.chunk(sid, "error:"+err.Error())
			return
		}
		a.chunk(sid, "wrote:"+path)
	}
}
