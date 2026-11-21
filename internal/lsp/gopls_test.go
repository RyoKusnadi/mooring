package lsp

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// goplsDir finds a real gopls on the PATH or in the Go bin folder.
func goplsDir(t *testing.T) string {
	t.Helper()
	if p, err := exec.LookPath("gopls"); err == nil {
		return filepath.Dir(p)
	}
	if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
		dir := filepath.Join(string(out[:len(out)-1]), "bin")
		if _, err := os.Stat(filepath.Join(dir, "gopls")); err == nil {
			return dir
		}
	}
	t.Skip("gopls is not installed")
	return ""
}

func TestRealGopls(t *testing.T) {
	dir := goplsDir(t)
	e := newEnv(t, nil, func(c *Config) { c.ExtraDirs = []string{dir} })
	e.write("go.mod", "module example.test/m\n\ngo 1.21\n")
	file := e.write("main.go", "package main\n\nfunc main() {\n\tvar x int\n}\n")
	e.allow("go")
	c := e.connect("go", "")
	res := c.handshake(1, map[string]any{"processId": nil, "capabilities": map[string]any{}})
	if r, _ := res["result"].(map[string]any); r["capabilities"] == nil {
		t.Fatal("initialize result has no capabilities")
	}
	uri := uriOf(file)
	c.open(uri, "package main\n\nfunc main() {\n\tvar x int\n}\n")
	d := c.diagnostics(uri, func(d []any) bool { return len(d) > 0 })
	if len(d) == 0 {
		t.Fatal("gopls reported nothing for an unused variable")
	}
}
