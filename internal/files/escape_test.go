package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Every operation refuses a path that leaves the project through a link.
func TestSymlinkEscapeRefused(t *testing.T) {
	r := newRig(t, func(c *Config) { c.RG = "-" })
	r.link(r.outside, "out")                                  // folder link
	r.link(filepath.Join(r.outside, "secret.txt"), "sec.txt") // file link
	r.link(filepath.Join(r.outside, "missing.txt"), "dangling.txt")
	r.write("inside.txt", "ok")
	r.link("inside.txt", "ok-link.txt")
	r.write("sub/a.txt", "a")

	_, err := r.s.List("p1", "out")
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Read("p1", "sec.txt", 0, 0, "")
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Read("p1", "out/secret.txt", 0, 0, "")
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Read("p1", "../outside/secret.txt", 0, 0, "")
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Write("p1", "sec.txt", Version([]byte("TOPSECRET needle\n")), []byte("pwned"))
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Write("p1", "out/new.txt", "", []byte("pwned"))
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Write("p1", "dangling.txt", "", []byte("pwned"))
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Create("p1", "out/new.txt", "file")
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Rename("p1", "inside.txt", "out/moved.txt")
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Rename("p1", "out/secret.txt", "taken.txt")
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Delete("p1", "out/secret.txt", true)
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Delete("p1", "../outside/secret.txt", true)
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "h", Dirs: []string{"out"}})
	r.wantCode(err, "OUTSIDE_PROJECT")
	_, err = r.s.Watch(WatchArgs{ProjectID: "p1", Holder: "h", Files: []string{"sec.txt"}})
	r.wantCode(err, "OUTSIDE_PROJECT")
	if b, _ := os.ReadFile(filepath.Join(r.outside, "secret.txt")); string(b) != "TOPSECRET needle\n" {
		t.Fatalf("outside file changed: %q", b)
	}
	if _, err := os.Stat(filepath.Join(r.outside, "new.txt")); err == nil {
		t.Fatal("file created outside")
	}

	// The listing shows the links as unusable, and a link inside the project as usable.
	res, err := r.s.List("p1", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Entries {
		switch e.Name {
		case "out", "sec.txt":
			if e.Unsafe != "outside" || e.Kind != "link" {
				t.Errorf("%s: %+v", e.Name, e)
			}
		case "dangling.txt":
			if e.Unsafe != "broken" {
				t.Errorf("dangling: %+v", e)
			}
		case "ok-link.txt":
			if e.Unsafe != "" || e.Kind != "file" || !e.Link {
				t.Errorf("ok-link: %+v", e)
			}
		}
	}

	// Search never reports the outside file, with either engine.
	for _, rg := range []string{"-", ""} {
		rr := newRig(t, func(c *Config) { c.RG = rg })
		rr.write("here.txt", "needle here\n")
		rr.link(rr.outside, "out")
		rr.link(filepath.Join(rr.outside, "secret.txt"), "sec.txt")
		lines := runSearch(t, rr, SearchRequest{ProjectID: "p1", Query: "needle"})
		for _, l := range lines {
			if l.T == "match" && l.Path != "here.txt" {
				t.Errorf("rg=%q: result outside the project: %+v", rg, l)
			}
		}
		if len(lines) != 2 {
			t.Errorf("rg=%q: lines %+v", rg, lines)
		}
	}
}

// Deleting or renaming a link acts on the link, never on its target.
func TestLinkOperationsStayInside(t *testing.T) {
	r := newRig(t, nil)
	r.link(r.outside, "out")
	if _, err := r.s.Rename("p1", "out", "out2"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.Delete("p1", "out2", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.outside, "secret.txt")); err != nil {
		t.Fatalf("target was touched: %v", err)
	}
}

func TestDeleteOutsideProjectRefused(t *testing.T) {
	r := newRig(t, nil)
	for _, p := range []string{"../outside/secret.txt", "../outside", "a/../../outside/secret.txt"} {
		_, err := r.s.Delete("p1", p, true)
		if c := code(err); c != "OUTSIDE_PROJECT" && c != "PROTECTED" {
			t.Errorf("%s: %q", p, c)
		}
	}
	if _, err := os.Stat(filepath.Join(r.outside, "secret.txt")); err != nil {
		t.Fatal(err)
	}
	_ = context.Background
}
