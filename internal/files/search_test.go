package files

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func runSearch(t *testing.T, r *rig, req SearchRequest) []SearchLine {
	t.Helper()
	var out []SearchLine
	err := r.s.Search(context.Background(), req, func(l SearchLine) error {
		out = append(out, l)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func searchFixture(r *rig) {
	r.write(".gitignore", "ignored/\n*.tmp\n")
	r.write("a.txt", "one needle two\nnothing\nNEEDLE again and needle\n")
	r.write("src/b.go", "package b // needle\n")
	r.write("ignored/c.txt", "needle\n")
	r.write("x.tmp", "needle\n")
	r.write("node_modules/m.js", "needle\n")
	r.write(".hidden/h.txt", "needle\n")
	r.write("bin.dat", "needle\x00\x01")
	r.write("long.txt", strings.Repeat("x", 500)+" needle "+strings.Repeat("y", 500)+"\n")
}

func TestSearchEngines(t *testing.T) {
	engines := map[string]string{"walk": "-"}
	if _, err := exec.LookPath("rg"); err == nil {
		engines["rg"] = ""
	} else {
		t.Log("rg not on PATH; only the Go walk is tested")
	}
	for name, rg := range engines {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, func(c *Config) { c.RG = rg })
			searchFixture(r)
			lines := runSearch(t, r, SearchRequest{ProjectID: "p1", Query: "needle"})
			got := map[string]int{}
			var done SearchLine
			for i, l := range lines {
				if l.Seq != i+1 {
					t.Errorf("seq %d at %d", l.Seq, i)
				}
				if l.T == "match" {
					got[l.Path]++
				} else {
					done = l
				}
			}
			want := map[string]int{"a.txt": 2, "src/b.go": 1, "long.txt": 1}
			for p, n := range want {
				if got[p] != n {
					t.Errorf("%s: %d matches, want %d (%v)", p, got[p], n, got)
				}
			}
			if len(got) != len(want) {
				t.Errorf("extra results: %v", got)
			}
			if done.T != "done" || done.Engine != name || done.Truncated {
				t.Errorf("done: %+v", done)
			}
			for _, l := range lines {
				if l.Path == "a.txt" && l.Line == 3 && (l.Col != 1 || len(l.Ranges) != 2 || l.Ranges[1] != [2]int{17, 23}) {
					t.Errorf("columns: %+v", l)
				}
				if l.Path == "long.txt" && (len(l.Text) > maxLineUnits || len(l.Ranges) != 1 || l.Text[l.Ranges[0][0]:l.Ranges[0][1]] != "needle") {
					t.Errorf("long line: len=%d %+v", len(l.Text), l.Ranges)
				}
			}
			// case-sensitive, regex, glob, folder, limit, bad pattern
			if n := countMatches(runSearch(t, r, SearchRequest{ProjectID: "p1", Query: "NEEDLE", Case: true})); n != 1 {
				t.Errorf("case: %d", n)
			}
			if n := countMatches(runSearch(t, r, SearchRequest{ProjectID: "p1", Query: `ne+dle\b`, Regex: true, Glob: "*.go"})); n != 1 {
				t.Errorf("regex+glob: %d", n)
			}
			if n := countMatches(runSearch(t, r, SearchRequest{ProjectID: "p1", Query: "needle", Path: "src"})); n != 1 {
				t.Errorf("folder: %d", n)
			}
			l := runSearch(t, r, SearchRequest{ProjectID: "p1", Query: "needle", Limit: 2})
			if countMatches(l) != 2 || !l[len(l)-1].Truncated {
				t.Errorf("limit: %+v", l)
			}
			bad := runSearch(t, r, SearchRequest{ProjectID: "p1", Query: "(", Regex: true})
			if len(bad) != 1 || bad[0].Code != "BAD_QUERY" {
				t.Errorf("bad pattern: %+v", bad)
			}
			esc := runSearch(t, r, SearchRequest{ProjectID: "p1", Query: "needle", Path: "../outside"})
			if len(esc) != 1 || esc[0].Code != "OUTSIDE_PROJECT" {
				t.Errorf("outside folder: %+v", esc)
			}
		})
	}
}

func countMatches(ls []SearchLine) int {
	n := 0
	for _, l := range ls {
		if l.T == "match" {
			n++
		}
	}
	return n
}

func TestSearchCancelAndBusy(t *testing.T) {
	r := newRig(t, func(c *Config) { c.RG = "-" })
	searchFixture(r)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := r.s.Search(ctx, SearchRequest{ProjectID: "p1", Query: "needle"}, func(SearchLine) error { return nil })
	if err == nil {
		t.Fatal("a cancelled search should return the cancellation")
	}
	for range maxSearches {
		r.s.srch <- struct{}{}
	}
	l := runSearch(t, r, SearchRequest{ProjectID: "p1", Query: "needle"})
	if len(l) != 1 || l[0].Code != "SEARCH_BUSY" {
		t.Fatalf("busy: %+v", l)
	}
}
