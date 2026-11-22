package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// syncBuffer is a bytes.Buffer that the server and the test can use at once.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// running is a server started on a free port by the real start path.
type running struct {
	base   string
	stop   func() error
	stderr *syncBuffer
}

func start(t *testing.T, dir, token string) *running {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	out, errOut := &syncBuffer{}, &syncBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, serveOpts{addr: "127.0.0.1:0", dir: dir, token: token, lspIdle: time.Minute, lspMemory: 2048, quiet: true}, out, errOut)
	}()
	re := regexp.MustCompile(`running at (http://\S+)`)
	deadline := time.Now().Add(20 * time.Second)
	for {
		if m := re.FindStringSubmatch(out.String()); m != nil {
			var once sync.Once
			var result error
			return &running{base: m[1], stderr: errOut, stop: func() error {
				once.Do(func() { cancel(); result = <-done })
				return result
			}}
		}
		select {
		case err := <-done:
			cancel()
			t.Fatalf("server stopped while starting: %v\n%s", err, errOut.String())
		case <-time.After(20 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("server did not start\n%s", errOut.String())
		}
	}
}

// loadFixture writes the fixture's database and token into a new data folder.
func loadFixture(t *testing.T, fixture string) (dir, token string) {
	t.Helper()
	dir = t.TempDir()
	src := filepath.Join("testdata", "upgrade", fixture)
	dump, err := os.ReadFile(filepath.Join(src, "mooring.sql"))
	if err != nil {
		t.Fatal(err)
	}
	tok, err := os.ReadFile(filepath.Join(src, "token"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "mooring.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(strings.ReplaceAll(string(dump), "{{ROOT}}", dir)); err != nil {
		t.Fatalf("load fixture %s: %v", fixture, err)
	}
	token = strings.TrimSpace(string(tok))
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, token
}

// rowCounts counts the rows of every table named in tables.
func rowCounts(t *testing.T, file string, tables []string) map[string]int {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+file+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	out := map[string]int{}
	for _, name := range tables {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "` + name + `"`).Scan(&n); err != nil {
			t.Fatalf("count %s in %s: %v", name, file, err)
		}
		out[name] = n
	}
	return out
}

func tableNames(t *testing.T, file string) []string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+file+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		names = append(names, n)
	}
	return names
}

func backupFiles(t *testing.T, dir string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, "backups", "*.db"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func get(t *testing.T, c *http.Client, u string) (int, string) {
	t.Helper()
	res, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body)
}

// welcome opens a socket, sends hello and returns the first frame's snapshot.
func welcome(t *testing.T, c *http.Client, base, path string) json.RawMessage {
	t.Helper()
	u, _ := url.Parse(base)
	cookies := c.Jar.Cookies(u)
	if len(cookies) == 0 {
		t.Fatal("no session cookie")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+path,
		&websocket.DialOptions{HTTPHeader: http.Header{"Cookie": {cookies[0].String()}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.CloseNow() }()
	if err := ws.Write(ctx, websocket.MessageText, []byte(`{"t":"hello","epoch":"","pos":0}`)); err != nil {
		t.Fatal(err)
	}
	_, data, err := ws.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		T        string          `json:"t"`
		Snapshot json.RawMessage `json:"snapshot"`
	}
	if err := json.Unmarshal(data, &f); err != nil || f.T != "welcome" {
		t.Fatalf("%s: %v %s", path, err, data)
	}
	return f.Snapshot
}

func TestUpgradeFromSchema6(t *testing.T) {
	dir, token := loadFixture(t, "schema-6")
	dbFile := filepath.Join(dir, "mooring.db")
	tables := tableNames(t, dbFile)
	before := rowCounts(t, dbFile, tables)
	if before["schema_migrations"] != 6 || before["projects"] != 2 || before["chat_messages"] != 3 {
		t.Fatalf("fixture is not what the test expects: %v", before)
	}

	srv := start(t, dir, token)
	defer func() { _ = srv.stop() }()

	// Every migration this build has is applied, and one backup holds the old schema.
	files, err := os.ReadDir(filepath.Join("..", "..", "internal", "store", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if after := rowCounts(t, dbFile, []string{"schema_migrations"}); after["schema_migrations"] != len(files) {
		t.Fatalf("migrations applied: %d, build has %d", after["schema_migrations"], len(files))
	}
	backups := backupFiles(t, dir)
	if len(backups) != 1 || !strings.HasPrefix(filepath.Base(backups[0]), "mooring-schema6-") {
		t.Fatalf("backups after the first start: %v", backups)
	}
	if got := rowCounts(t, backups[0], tables); fmt.Sprint(got) != fmt.Sprint(before) {
		t.Fatalf("backup rows %v, fixture rows %v", got, before)
	}
	if got := rowCounts(t, dbFile, []string{"projects", "chats", "chat_messages", "accounts", "tool_servers"}); got["projects"] != 2 || got["chats"] != 2 || got["chat_messages"] != 3 || got["accounts"] != 2 || got["tool_servers"] != 2 {
		t.Fatalf("rows after migrating: %v", got)
	}

	// Login with the fixture's token, then read what the browser reads.
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	res, err := c.Post(srv.base+"/api/login", "application/json", strings.NewReader(`{"token":"`+token+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("login: %d", res.StatusCode)
	}
	var snap struct {
		Projects []struct{ ID, Name string } `json:"projects"`
		Chats    map[string][]struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"chats"`
	}
	if err := json.Unmarshal(welcome(t, c, srv.base, "/api/events"), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Projects) != 2 || snap.Projects[0].ID != "p1" || len(snap.Chats["p1"]) != 2 {
		t.Fatalf("snapshot: %+v", snap)
	}
	var chat struct {
		Messages []struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(welcome(t, c, srv.base, "/api/chats/c1"), &chat); err != nil {
		t.Fatal(err)
	}
	if len(chat.Messages) != 3 || chat.Messages[0].Kind != "user" || !strings.Contains(chat.Messages[1].Text, "reads the clock") {
		t.Fatalf("chat replay: %+v", chat)
	}
	if code, body := get(t, c, srv.base+"/api/accounts"); code != 200 || !strings.Contains(body, `"acct-work"`) {
		t.Fatalf("accounts: %d %s", code, body)
	}
	if code, body := get(t, c, srv.base+"/api/settings"); code != 200 || !strings.Contains(body, "compactDefault") {
		t.Fatalf("settings: %d %s", code, body)
	}

	// A second start has nothing to migrate and makes no backup.
	if err := srv.stop(); err != nil {
		t.Fatalf("stop: %v\n%s", err, srv.stderr.String())
	}
	again := start(t, dir, token)
	defer func() { _ = again.stop() }()
	if got := backupFiles(t, dir); len(got) != 1 {
		t.Fatalf("backups after the second start: %v", got)
	}
}
