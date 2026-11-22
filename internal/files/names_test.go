package files

import (
	"context"
	"os/exec"
	"slices"
	"testing"
)

func TestNamesEngines(t *testing.T) {
	engines := map[string]string{"walk": "-"}
	if _, err := exec.LookPath("rg"); err == nil {
		engines["rg"] = ""
	}
	for name, rg := range engines {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, func(c *Config) { c.RG = rg })
			r.write("a.txt", "x")
			r.write(".git/HEAD", "x")
			r.write("src/b.go", "x")
			r.write(".hidden/c.txt", "x")
			r.write("node_modules/d.js", "x")
			r.write("gen/e.txt", "x")
			r.write(".gitignore", "gen/\n")
			r.write(r.outside+"/secret.txt", "x")
			r.link(r.outside, "escape")
			res, err := r.s.Names(context.Background(), "p1")
			if err != nil {
				t.Fatal(err)
			}
			if want := []string{".gitignore", ".hidden/c.txt", "a.txt", "src/b.go"}; !slices.Equal(res.Paths, want) || res.Truncated || res.Engine != name {
				t.Fatalf("got %+v, want %v from %s", res, want, name)
			}
		})
	}
}
