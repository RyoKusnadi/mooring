package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// bigHistory fills the rig's repository with n commits on main and four side
// branches, a merge every 21st commit, through git fast-import.
func bigHistory(t *testing.T, r *rig, n int) {
	t.Helper()
	var b bytes.Buffer
	side, merged := 0, 0
	for i := 1; i <= n; i++ {
		if i%7 == 0 {
			side = i + 1000000
			sm := fmt.Sprintf("side %d", i)
			fmt.Fprintf(&b, "commit refs/heads/side%d\nmark :%d\nauthor Side <side@example.com> %d +0000\ncommitter Side <side@example.com> %d +0000\ndata %d\n%s\nfrom :%d\nM 100644 inline s.txt\ndata 2\ns\n\n", i%4, side, 1_000_000_000+i, 1_000_000_000+i, len(sm), sm, i-1)
		}
		msg := fmt.Sprintf("commit %d", i)
		fmt.Fprintf(&b, "commit refs/heads/main\nmark :%d\nauthor Ann <ann@example.com> %d +0000\ncommitter Ann <ann@example.com> %d +0000\ndata %d\n%s\n", i, 1_000_000_000+i, 1_000_000_000+i, len(msg), msg)
		if i > 1 {
			fmt.Fprintf(&b, "from :%d\n", i-1)
		}
		if i%21 == 0 && side != merged {
			fmt.Fprintf(&b, "merge :%d\n", side)
			merged = side
		}
		fmt.Fprintf(&b, "M 100644 inline f.txt\ndata %d\n%d\n\n", len(strconv.Itoa(i))+1, i)
	}
	fmt.Fprintf(&b, "reset refs/tags/v1\nfrom :500\n\n")
	cmd := exec.Command("git", "fast-import", "--quiet", "--done")
	cmd.Dir = r.root
	cmd.Env = append(os.Environ(), r.env...)
	b.WriteString("done\n")
	cmd.Stdin = &b
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fast-import: %v\n%s", err, out)
	}
	r.git(r.root, "reset", "-q", "--hard", "main")
}

func TestLogHundredThousand(t *testing.T) {
	r := newRig(t, opts{noCommit: true})
	bigHistory(t, r, 100_000)
	want, _ := strconv.Atoi(r.git(r.root, "rev-list", "--all", "--count"))
	start := time.Now()
	page, err := r.s.Log(bg, "p1", "", LogQuery{})
	r.must(err)
	first := time.Since(start)
	if len(page.Commits) != 200 || page.Total != want || page.Next == "" {
		t.Fatalf("first page: %d commits, total %d (want %d), next %q", len(page.Commits), page.Total, want, page.Next)
	}
	if first > 2*time.Second {
		t.Errorf("first page took %v", first)
	}
	pos := map[string]int{}
	var all []LogCommit
	pages := 0
	paging := time.Now()
	for {
		for _, c := range page.Commits {
			pos[c.ID] = len(all)
			all = append(all, c)
		}
		pages++
		if page.Next == "" {
			break
		}
		if page, err = r.s.Log(bg, "p1", "", LogQuery{Cursor: page.Next, Limit: 500}); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d commits, first page %v, %d pages in %v", len(all), first.Round(time.Millisecond), pages, time.Since(paging).Round(time.Millisecond))
	if len(all) != want || len(pos) != want {
		t.Fatalf("paged %d commits (%d distinct), want %d", len(all), len(pos), want)
	}
	for i, c := range all {
		for _, p := range c.Parents {
			if j, ok := pos[p]; !ok || j <= i {
				t.Fatalf("commit %d: parent %s at %d (found %v)", i, p, j, ok)
			}
		}
	}
	// A new commit moves the refs: the old cursor is stale.
	first2, err := r.s.Log(bg, "p1", "", LogQuery{})
	r.must(err)
	r.write("g.txt", "g\n")
	r.commitAll("one more")
	_, err = r.s.Log(bg, "p1", "", LogQuery{Cursor: first2.Next})
	r.wantCode(err, "LOG_CHANGED")
	again, err := r.s.Log(bg, "p1", "", LogQuery{})
	r.must(err)
	if again.Total != want+1 || again.Commits[0].Subject != "one more" {
		t.Fatalf("after commit: total %d, top %q", again.Total, again.Commits[0].Subject)
	}
}

func TestLogRefsAndShape(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.txt", "a\n")
	r.commitAll("second")
	r.git(r.root, "tag", "v1")
	r.git(r.root, "tag", "-a", "-m", "note", "v2")
	r.git(r.root, "branch", "topic", "HEAD~1")
	page, err := r.s.Log(bg, "p1", "", LogQuery{})
	r.must(err)
	if page.Total != 2 || page.Next != "" || len(page.Commits) != 2 {
		t.Fatalf("page %+v", page)
	}
	top, root := page.Commits[0], page.Commits[1]
	if top.Subject != "second" || top.Author != "Test" || top.Email != "test@example.com" || len(top.Parents) != 1 || top.Parents[0] != root.ID {
		t.Fatalf("top %+v", top)
	}
	if got := fmt.Sprint(top.Refs); got != "[{main head} {v1 tag} {v2 tag}]" {
		t.Fatalf("refs %s", got)
	}
	if got := fmt.Sprint(root.Refs); got != "[{topic branch}]" || len(root.Parents) != 0 {
		t.Fatalf("root %+v", root)
	}
	r.git(r.root, "checkout", "-q", "--detach")
	page, err = r.s.Log(bg, "p1", "", LogQuery{Limit: 1})
	r.must(err)
	if fmt.Sprint(page.Commits[0].Refs[0]) != "{HEAD head}" || page.Next == "" {
		t.Fatalf("detached %+v", page)
	}
	_, err = r.s.Log(bg, "p1", "", LogQuery{Limit: 501})
	r.wantCode(err, "BAD_REQUEST")
	_, err = r.s.Log(bg, "p1", "", LogQuery{Cursor: "nonsense"})
	r.wantCode(err, "BAD_REQUEST")
	empty := newRig(t, opts{noCommit: true})
	page, err = empty.s.Log(bg, "p1", "", LogQuery{})
	empty.must(err)
	if len(page.Commits) != 0 || page.Total != 0 || page.Next != "" {
		t.Fatalf("empty %+v", page)
	}
	page, err = empty.s.Log(bg, "p1", "", LogQuery{Text: "x", In: "message"})
	empty.must(err)
	if len(page.Commits) != 0 {
		t.Fatal("search in empty")
	}
}

func TestLogSearch(t *testing.T) {
	r := newRig(t, opts{})
	r.write("a.txt", "alpha needle\n")
	r.commitAll("Fix the Parser (regex.*)")
	r.git(r.root, "-c", "user.name=Zoe Quinn", "-c", "user.email=zoe@other.org", "commit", "-q", "--allow-empty", "-m", "body only\n\nmentions Widgets here")
	r.git(r.root, "checkout", "-q", "-b", "other")
	r.write("b.txt", "beta\n")
	r.commitAll("on other branch")
	r.git(r.root, "checkout", "-q", "main")
	find := func(in, text string) []string {
		page, err := r.s.Log(bg, "p1", "", LogQuery{Text: text, In: in})
		r.must(err)
		if page.Total != -1 || page.Stopped {
			t.Fatalf("%s %q: %+v", in, text, page)
		}
		var subj []string
		for _, c := range page.Commits {
			subj = append(subj, c.Subject)
		}
		return subj
	}
	for _, tc := range []struct{ in, text, want string }{
		{"message", "fix the parser", "[Fix the Parser (regex.*)]"},
		{"message", "regex.*", "[Fix the Parser (regex.*)]"},
		{"message", "a.*", "[]"},
		{"message", "WIDGETS", "[body only]"},
		{"author", "zoe quinn", "[body only]"},
		{"author", "OTHER.ORG", "[body only]"},
		{"author", "Zoe.*", "[]"},
		{"code", "needle", "[Fix the Parser (regex.*)]"},
		{"code", "beta", "[on other branch]"},
		{"code", "nothing", "[]"},
	} {
		if got := fmt.Sprint(find(tc.in, tc.text)); got != tc.want {
			t.Errorf("%s %q = %s, want %s", tc.in, tc.text, got, tc.want)
		}
	}
	// Paging a search: the cursor is an offset.
	p1, err := r.s.Log(bg, "p1", "", LogQuery{Text: "e", In: "message", Limit: 1})
	r.must(err)
	if len(p1.Commits) != 1 || p1.Next != "1" {
		t.Fatalf("p1 %+v", p1)
	}
	p2, err := r.s.Log(bg, "p1", "", LogQuery{Text: "e", In: "message", Limit: 1, Cursor: p1.Next})
	r.must(err)
	if len(p2.Commits) != 1 || p2.Commits[0].ID == p1.Commits[0].ID {
		t.Fatalf("p2 %+v", p2)
	}
	for _, q := range []LogQuery{{Text: "", In: "message", Cursor: "1"}, {Text: strings.Repeat("x", 201), In: "message"}, {Text: "x", In: "path"}, {Text: "x", In: "message", Cursor: "-1"}} {
		_, err := r.s.Log(bg, "p1", "", q)
		r.wantCode(err, "BAD_REQUEST")
	}
	old := searchTimeout
	searchTimeout = time.Nanosecond
	defer func() { searchTimeout = old }()
	page, err := r.s.Log(bg, "p1", "", LogQuery{Text: "needle", In: "code"})
	r.must(err)
	if !page.Stopped || page.Next != "" {
		t.Fatalf("timeout: %+v", page)
	}
}
